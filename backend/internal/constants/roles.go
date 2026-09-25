package constants
type Role string
const (
	RoleAdmin            Role = "admin"
	RoleProcessScientist Role = "process_scientist"
	RoleDataAnalyst      Role = "data_analyst"
	RoleReviewer         Role = "reviewer"
	RoleAuditor          Role = "auditor"
)
const (
	PermissionRead            = "read"
	PermissionVesselWrite     = "vessel:write"
	PermissionRecipeWrite     = "recipe:write"
	PermissionRecipePublish   = "recipe:publish"
	PermissionSeriesImport    = "series:import"
	PermissionSeriesProcess   = "series:process"
	PermissionAnalysisRun     = "analysis:run"
	PermissionAnalysisReview  = "analysis:review"
	PermissionAnalysisConfirm = "analysis:confirm"
	PermissionAuditRead       = "audit:read"
)
var rolePermissions = map[Role]map[string]struct{}{
	RoleAdmin: {
		PermissionRead: {}, PermissionVesselWrite: {}, PermissionRecipeWrite: {},
		PermissionRecipePublish: {}, PermissionSeriesImport: {}, PermissionSeriesProcess: {},
		PermissionAnalysisRun: {}, PermissionAnalysisReview: {}, PermissionAnalysisConfirm: {},
		PermissionAuditRead: {},
	},
	RoleProcessScientist: {
		PermissionRead: {}, PermissionVesselWrite: {}, PermissionRecipeWrite: {},
		PermissionRecipePublish: {}, PermissionSeriesProcess: {}, PermissionAnalysisReview: {},
	},
	RoleDataAnalyst: {
		PermissionRead: {}, PermissionSeriesImport: {}, PermissionSeriesProcess: {},
		PermissionAnalysisRun: {},
	},
	RoleReviewer: {
		PermissionRead: {}, PermissionAnalysisReview: {}, PermissionAnalysisConfirm: {},
		PermissionAuditRead: {},
	},
	RoleAuditor: {PermissionRead: {}, PermissionAuditRead: {}},
}
func (r Role) Valid() bool {
	_, ok := rolePermissions[r]
	return ok
}
func HasPermission(role Role, permission string) bool {
	permissions, ok := rolePermissions[role]
	if !ok {
		return false
	}
	_, ok = permissions[permission]
	return ok
}
func RoleValues() []string {
	return []string{
		string(RoleAdmin), string(RoleProcessScientist), string(RoleDataAnalyst),
		string(RoleReviewer), string(RoleAuditor),
	}
}
