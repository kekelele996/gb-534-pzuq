package main
import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"fermentation-kinetics-deviation-analysis/backend/internal/algorithm"
	"fermentation-kinetics-deviation-analysis/backend/internal/config"
	"fermentation-kinetics-deviation-analysis/backend/internal/constants"
	"fermentation-kinetics-deviation-analysis/backend/internal/database"
	"fermentation-kinetics-deviation-analysis/backend/internal/handler"
	"fermentation-kinetics-deviation-analysis/backend/internal/middleware"
	"fermentation-kinetics-deviation-analysis/backend/internal/repository"
	"fermentation-kinetics-deviation-analysis/backend/internal/router"
	"fermentation-kinetics-deviation-analysis/backend/internal/service"
	"fermentation-kinetics-deviation-analysis/backend/internal/util"
	"github.com/gin-gonic/gin"
)
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}
func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(db); closeErr != nil {
			logger.Error("database close failed", "error", closeErr)
		}
	}()
	vesselRepo := repository.NewFermentationVesselRepository(db)
	recipeRepo := repository.NewCultureRecipeRepository(db)
	seriesRepo := repository.NewSensorSeriesRepository(db)
	analysisRepo := repository.NewDeviationAnalysisRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	userRepo := repository.NewUserRepository(db)
	vesselHandler := handler.NewFermentationVesselHandler(service.NewFermentationVesselService(vesselRepo, auditRepo))
	recipeHandler := handler.NewCultureRecipeHandler(service.NewCultureRecipeService(recipeRepo, vesselRepo, auditRepo))
	seriesHandler := handler.NewSensorSeriesHandler(service.NewSensorSeriesService(seriesRepo, recipeRepo, vesselRepo, auditRepo))
	analysisHandler := handler.NewDeviationAnalysisHandler(service.NewDeviationAnalysisService(
		analysisRepo, recipeRepo, seriesRepo, auditRepo, algorithm.NewEvaluator(),
	))
	auth := middleware.NewAuthenticator(userRepo, cfg)
	loginLimiter := middleware.NewRateLimiter(cfg.LoginLimitPerMinute)
	importLimiter := middleware.NewRateLimiter(cfg.ImportLimitPerMinute)
	analysisLimiter := middleware.NewRateLimiter(cfg.AnalysisLimitPerMinute)
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recovery(logger), middleware.ErrorHandler(logger))
	engine.GET("/healthz", func(c *gin.Context) {
		sqlDB, dbErr := db.DB()
		if dbErr != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			util.Fail(c, util.NewError(http.StatusInternalServerError, util.CodeInternal, "database is unavailable"))
			return
		}
		util.Success(c, http.StatusOK, gin.H{
			"status": "healthy", "service": "fermentation-kinetics-deviation-analysis", "time": time.Now().UTC(),
		})
	})
	v1 := engine.Group("/api/v1")
	v1.POST("/auth/login", loginLimiter.Middleware("login"), auth.Login)
	api := v1.Group("")
	api.Use(auth.RequireAuth(), middleware.Audit(auditRepo))
	router.RegisterFermentationVesselRoutes(api, vesselHandler)
	router.RegisterCultureRecipeRoutes(api, recipeHandler)
	router.RegisterSensorSeriesRoutes(api, seriesHandler, importLimiter)
	router.RegisterDeviationAnalysisRoutes(api, analysisHandler, analysisLimiter)
	api.GET("/audit-logs", middleware.RequirePermission(constants.PermissionAuditRead), middleware.AuditListHandler(auditRepo))
	api.GET("/meta/enums", middleware.RequirePermission(constants.PermissionRead), func(c *gin.Context) {
		util.Success(c, http.StatusOK, gin.H{
			"fermentation_phases": constants.FermentationPhaseValues(),
			"deviation_levels":    constants.DeviationLevelValues(),
			"series_states":       constants.SeriesStateValues(),
			"recipe_states":       constants.RecipeStateValues(),
			"analysis_states":     constants.AnalysisStateValues(),
			"roles":               constants.RoleValues(),
		})
	})
	engine.NoRoute(func(c *gin.Context) {
		util.Fail(c, util.NewError(http.StatusNotFound, util.CodeNotFound, "route was not found"))
	})
	server := &http.Server{Addr: ":" + cfg.Port, Handler: engine, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	logger.Info("server started", "port", cfg.Port, "database_driver", cfg.DBDriver)
	select {
	case listenErr := <-serverErr:
		if !errors.Is(listenErr, http.ErrServerClosed) {
			return listenErr
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
	}
	return nil
}
