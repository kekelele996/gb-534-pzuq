export const deviationLevels = ['normal', 'watch', 'major', 'critical'] as const
export type DeviationLevel = typeof deviationLevels[number]
