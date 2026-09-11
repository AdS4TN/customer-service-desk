export type ReceptionField = { key: string; label: string; askWhen: string; required: boolean }
export type ReceptionPolicy = {
  enabled: boolean
  goal: string
  instructions: string
  handoffConditions: string
  fields: ReceptionField[]
}

export function emptyReceptionPolicy(): ReceptionPolicy {
  return { enabled: false, goal: "", instructions: "", handoffConditions: "", fields: [] }
}

export function receptionPolicyError(p: ReceptionPolicy) {
  if (p.enabled && !p.goal.trim()) return "goalRequired"
  if (p.fields.length > 16 || [...p.goal.trim()].length > 500 || [...p.instructions.trim()].length > 3000 || [...p.handoffConditions.trim()].length > 1500) return "invalid"
  const keys = new Set<string>()
  const labels = new Set<string>()
  for (const f of p.fields) {
    const label = f.label.trim().toLowerCase()
    if (!/^[a-z][a-z0-9_]{0,47}$/.test(f.key) || keys.has(f.key) || !label || labels.has(label) || [...f.label.trim()].length > 100 || [...f.askWhen.trim()].length > 500) return "fieldsInvalid"
    keys.add(f.key)
    labels.add(label)
  }
  return ""
}
