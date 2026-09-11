import type { ConversationMemory } from "@/lib/api/conversation-memory";

const failureKeys: Record<string, string> = {
  model_unavailable: "leadAnalysis.errors.modelUnavailable",
  model_request_failed: "leadAnalysis.errors.modelRequest",
  model_timeout: "leadAnalysis.errors.modelTimeout",
  memory_validation_failed: "leadAnalysis.errors.memoryValidation",
  inquiry_validation_failed: "leadAnalysis.errors.inquiryValidation",
  lead_validation_failed: "leadAnalysis.errors.leadValidation",
  context_load_failed: "leadAnalysis.errors.contextLoad",
  persistence_failed: "leadAnalysis.errors.persistence",
  worker_interrupted: "leadAnalysis.errors.interrupted",
};

export function leadAnalysisState(memory: ConversationMemory | null) {
  const pending = memory?.status === "queued" || memory?.status === "processing";
  return {
    pending,
    failed: memory?.status === "failed",
    retrying: pending && Boolean(memory?.errorCode),
    emptyKey: memory?.status === "ready"
      ? "leadAnalysis.noIntent"
      : memory?.status === "empty" ? "leadAnalysis.notAnalyzed" : null,
    errorKey: failureKeys[memory?.errorCode ?? ""] ?? "leadAnalysis.errors.unknown",
  };
}
