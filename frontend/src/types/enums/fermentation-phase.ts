export const fermentationPhases = ['lag', 'growth', 'production', 'harvest'] as const
export type FermentationPhase = typeof fermentationPhases[number]
