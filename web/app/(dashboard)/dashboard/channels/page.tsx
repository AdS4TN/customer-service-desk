"use client"

import { useEffect } from "react"
import { useRouter } from "next/navigation"

export default function DashboardChannelsPage() {
  const router = useRouter()
  useEffect(() => { router.replace("/dashboard/channels/web") }, [router])
  return null
}
