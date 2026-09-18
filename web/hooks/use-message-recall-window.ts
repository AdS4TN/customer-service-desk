"use client"

import { useEffect, useState } from "react"

export function isMessageRecallWindowOpen(recallableUntil?: string, now = Date.now()) {
  if (!recallableUntil) return false
  const deadline = Date.parse(recallableUntil)
  return Number.isFinite(deadline) && now <= deadline
}

export function useMessageRecallWindow(recallableUntil?: string) {
  const [expiredDeadline, setExpiredDeadline] = useState("")
  const isOpen =
    expiredDeadline !== recallableUntil && isMessageRecallWindowOpen(recallableUntil)

  useEffect(() => {
    const deadline = Date.parse(recallableUntil ?? "")
    if (!Number.isFinite(deadline) || deadline < Date.now() || expiredDeadline === recallableUntil) return

    const timeout = window.setTimeout(
      () => setExpiredDeadline(recallableUntil ?? ""),
      deadline - Date.now() + 25
    )
    return () => window.clearTimeout(timeout)
  }, [expiredDeadline, recallableUntil])

  return isOpen
}
