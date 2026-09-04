export function moduleCheckState(accessByUser, moduleCode) {
  if (!Array.isArray(accessByUser) || accessByUser.length === 0) {
    return 'unchecked'
  }

  const grantedCount = accessByUser.reduce((count, access) => {
    return count + (Array.isArray(access.moduleCodes) && access.moduleCodes.includes(moduleCode) ? 1 : 0)
  }, 0)

  if (grantedCount === accessByUser.length) return 'checked'
  if (grantedCount === 0) return 'unchecked'
  return 'indeterminate'
}

export function selectedControlledCodes(modules, selectedCodes) {
  const selected = new Set(selectedCodes || [])
  return (modules || [])
    .filter((module) => module.accessMode === 'user_allowlist' && selected.has(module.code))
    .map((module) => module.code)
}
