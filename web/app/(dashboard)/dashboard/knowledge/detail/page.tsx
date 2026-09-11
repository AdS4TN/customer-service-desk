"use client"

import { useRouter, useSearchParams } from "next/navigation"
import { useEffect } from "react"

export default function KnowledgeBaseDetailPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const knowledgeBaseId = Number(searchParams.get("id"))

  useEffect(() => {
    const suffix = Number.isInteger(knowledgeBaseId) && knowledgeBaseId > 0 ? `?id=${knowledgeBaseId}` : ""
    router.replace(`/dashboard/knowledge/ingest${suffix}`)
  }, [knowledgeBaseId, router])

  return null
}
