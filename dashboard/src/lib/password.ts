const BCRYPT_PREFIX_RE = /^\$2[aby]\$\d\d\$/

export async function hashPasswordIfNeeded(password: string): Promise<string> {
  if (BCRYPT_PREFIX_RE.test(password)) {
    return password
  }
  return password
}
