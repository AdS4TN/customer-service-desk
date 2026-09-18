export function normalizeCustomerTags(values: string[]): string[] {
  const seen = new Set<string>()
  return values.map((value) => value.trim()).filter((value) => {
    const key = value.toLowerCase()
    if (!value || seen.has(key)) return false
    seen.add(key)
    return true
  })
}
