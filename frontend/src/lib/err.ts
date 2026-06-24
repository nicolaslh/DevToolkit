// Normalizes a rejected Wails binding call into a user-facing message.
// Go errors (including *apperr.AppError) arrive as a JS Error whose message
// is the Go error string (the Chinese AppError.Message).
export function errMsg(e: unknown): string {
  if (e instanceof Error && e.message) return e.message;
  if (typeof e === "string") return e;
  try {
    return String(e);
  } catch {
    return "发生未知错误";
  }
}
