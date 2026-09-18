import { request, requestBlob } from "@/lib/api/client";
import type { DashboardCrudQueryValue } from "@/components/dashboard/crud";

export type ExperienceMessage = {
  id: number;
  conversationId: number;
  role: string;
  type: string;
  text: string;
  at: string;
};
export type ExperienceSnapshot = {
  schemaVersion: string;
  customerId: number;
  customerName: string;
  conversationIds: number[];
  messages: ExperienceMessage[];
};
export type ExperienceSource = {
  id: number;
  customerId: number;
  customerName: string;
  channelType: string;
  channelName: string;
  lastMessageAt: string;
  messageCount: number;
  status: number;
};
export type ExperienceCase = {
  id: number;
  name: string;
  customerId: number;
  sourceHash: string;
  messageCount: number;
  outcome: string;
  outcomeNote: string;
  extractionStatus: "" | "running" | "succeeded" | "no_skill" | "failed";
  extractionModel: string;
  extractionResult?: SkillMinerDebugResult;
  extractedAt?: string;
  createdAt: string;
  snapshot?: ExperienceSnapshot;
};
export type ExperienceImportPreview = {
  conversationId: number;
  customerId: number;
  customerName: string;
  messages: ExperienceMessage[];
};
export type ExperienceImportSelection = {
  conversationId: number;
  includeAll: boolean;
  messageIds?: number[];
};
export type ExperienceRule = {
  id: string;
  condition: string;
  action: string;
  exceptions: string;
};
export type ExperienceEvent = {
  id: string;
  title: string;
  signal: string;
  sellerAction: string;
  observedResult: string;
  uncertainty: string;
  skillIds: string[];
  quotes: { messageId: number; text: string }[];
};
export type ExperienceChange = {
  skillId: string;
  operation: string;
  rule: ExperienceRule | null;
  eventIds: string[];
  rationale: string;
};
export type ExperiencePayload = {
  rules: ExperienceRule[];
  evidence: {
    caseId: number;
    sourceHash: string;
    ruleId: string;
    events: ExperienceEvent[];
  }[];
  changes: ExperienceChange[];
  provenance: string;
};
export type ExperienceRevision = {
  id: number;
  skillId: string;
  parentId: number;
  jobId: number;
  hash: string;
  note: string;
  payload: ExperiencePayload;
  createdAt: string;
};
export type ExperienceOptions = {
  skills: { id: string; activeRevisionId: number }[];
  models: { id: number; name: string; modelName: string }[];
};
export type SkillMinerEvidenceQuote = {
  messageId: number;
  quote: string;
};
export type SkillMinerEvidence = {
  customerBefore: SkillMinerEvidenceQuote[];
  sellerMove: SkillMinerEvidenceQuote[];
  customerAfter: SkillMinerEvidenceQuote[];
};
export type SkillMinerSkill = {
  name: string;
  description: string;
  whenToUse: string[];
  objective: string;
  steps: { instruction: string; purpose: string }[];
  successSignals: string[];
  whenNotToUse: string[];
};
export type SkillMinerCandidate = {
  sourceEpisodeId: string;
  skill: SkillMinerSkill;
  evidence: SkillMinerEvidence;
  assessment: {
    customerChange: string;
    causalReason: string;
    transferReason: string;
    confidence: "medium" | "high";
  };
};
export type SkillMinerResult = {
  protocolVersion: string;
  hasLearnableSkill: boolean;
  assessment: {
    conversationNature: "routine_inquiry" | "sales_episode" | "mixed";
    reason: string;
  };
  candidates: SkillMinerCandidate[];
};
export type SkillMinerDebugResult = {
  ok: boolean;
  model?: string;
  error?: string;
  raw?: string;
  result?: SkillMinerResult;
  candidateReviews?: Record<
    string,
    {
      status: "confirmed" | "dismissed";
      skillDefinitionId?: number;
      reviewedAt: string;
    }
  >;
  reviewStatus?: "confirmed" | "dismissed" | "mixed";
  reviewedAt?: string;
  skillDefinitionIds?: number[];
};
export type SkillComparePreview = {
  content: string;
  modelName: string;
  knowledgeStatus: string;
  mountedSkills: { id: number; name: string }[];
  durationMs: number;
};
export type SkillCompareResult = {
  aiAgentId: number;
  aiAgentName: string;
  skillDefinitionId: number;
  skillName: string;
  withoutSkill: SkillComparePreview;
  withSkill: SkillComparePreview;
};
export type ExperienceDelta = {
  conflict: boolean;
  current: ExperienceRule | null;
  ruleId: string;
  kind: "add" | "revise" | "support" | "remove";
  before: ExperienceRule | null;
  after: ExperienceRule | null;
  evidence: ExperiencePayload["evidence"];
  rationale: string[];
};
export type ExperienceWorkspace = {
  skillId: string;
  current: ExperienceRevision;
  suggestions: {
    id: number;
    jobId: number;
    createdAt: string;
    changes: ExperienceDelta[];
  }[];
};
export type ExperienceJob = {
  id: number;
  kind: string;
  state: string;
  stage: string;
  errorCode: string;
  attempts: number;
  modelConfigId: number;
  modelName?: string;
  rating: string;
  ratingNote: string;
  createdAt: string;
  updatedAt: string;
  callStartedAt?: string;
  heartbeatAt?: string;
  lastResponseAt?: string;
  outputChars?: number;
  callPhase?: string;
  input?: {
    cases: {
      id: number;
      sourceHash: string;
      snapshot?: ExperienceSnapshot;
      outcome?: string;
    }[];
    skillId: string;
    revisionId?: number;
    cutoffId: number;
    prefix: ExperienceMessage[];
    businessContext: string;
    mountedLabel: string;
    includeAI: boolean;
  };
  output?: {
    cases:
      | {
          caseId: number;
          summary: string;
          events: ExperienceEvent[];
          windowDone: number;
          windowTotal: number;
          proposed: boolean;
          skipped?: number;
        }[]
      | null;
    revisionIds: number[] | null;
    replies: Record<string, string> | null;
  };
};
export type ExperiencePage<T> = {
  results: T[];
  page: { page: number; limit: number; total: number };
};
const base = "/api/dashboard/sales-experience";
const post = <T>(path: string, body: unknown) =>
  request<T>(base + path, { method: "POST", body: JSON.stringify(body) });
function queryString(query: Record<string, DashboardCrudQueryValue>) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query))
    if (value !== undefined && value !== null) params.set(key, String(value));
  return params.toString();
}
export const experienceAPI = {
  skills: () => request<ExperiencePage<ExperienceWorkspace>>(base + "/skills"),
  skill: (skill: string) =>
    request<ExperienceWorkspace>(
      base + "/skill?skill=" + encodeURIComponent(skill),
    ),
  review: (
    proposalId: number,
    ruleId: string,
    decision: "accept" | "ignore",
    expectedId: number,
    rule?: ExperienceRule,
  ) =>
    post<void>("/review", { proposalId, ruleId, decision, expectedId, rule }),
  reviewBatch: (
    skillId: string,
    expectedId: number,
    items: { proposalId: number; ruleId: string; decision: "accept" }[],
  ) => post<void>("/review_batch", { skillId, expectedId, items }),
  saveSkill: (skillId: string, expectedId: number, rules: ExperienceRule[]) =>
    post<void>("/save_skill", { skillId, expectedId, rules }),
  options: () => request<ExperienceOptions>(base + "/options"),
  sources: (q: Record<string, DashboardCrudQueryValue>) =>
    request<ExperiencePage<ExperienceSource>>(
      base + "/sources?" + queryString(q),
    ),
  cases: (q: Record<string, DashboardCrudQueryValue>) =>
    request<ExperiencePage<ExperienceCase>>(base + "/cases?" + queryString(q)),
  case: (id: number) => request<ExperienceCase>(base + `/case?id=${id}`),
  deleteCase: (id: number) => post<void>("/case/delete", { id }),
  revisions: (q: Record<string, DashboardCrudQueryValue>) =>
    request<ExperiencePage<ExperienceRevision>>(
      base + "/revisions?" + queryString(q),
    ),
  revision: (id: number) =>
    request<ExperienceRevision>(base + `/revision?id=${id}`),
  jobs: (q: Record<string, DashboardCrudQueryValue>) =>
    request<ExperiencePage<ExperienceJob>>(base + "/jobs?" + queryString(q)),
  job: (id: number) => request<ExperienceJob>(base + `/job?id=${id}`),
  importPreview: (conversationIds: number[]) =>
    post<ExperienceImportPreview[]>("/import_preview", { conversationIds }),
  import: (selections: ExperienceImportSelection[]) =>
    post<ExperienceCase[]>("/import", { selections }),
  annotate: (id: number, outcome: string, note: string) =>
    post<void>("/annotate", { id, outcome, note }),
  distill: (caseIds: number[], modelConfigId: number, includeAI: boolean) =>
    post<ExperienceJob>("/distill", { caseIds, modelConfigId, includeAI }),
  evaluate: (body: {
    caseId: number;
    skillId: string;
    revisionId?: number;
    cutoffId: number;
    modelConfigId: number;
    businessContext: string;
  }) => post<ExperienceJob>("/evaluate", body),
  edit: (revisionId: number, rules: ExperienceRule[], note: string) =>
    post<ExperienceRevision>("/edit", { revisionId, rules, note }),
  activate: (id: number) => post<void>("/activate", { id }),
  retry: (id: number) => post<void>("/retry", { id }),
  cancel: (id: number) => post<void>("/cancel", { id }),
  rate: (id: number, rating: string, note: string) =>
    post<void>("/rate", { id, rating, note }),
  skillMinerDebug: (caseId: number, modelConfigId?: number) =>
    post<SkillMinerDebugResult>("/skill-miner-debug", { caseId, modelConfigId }),
  reviewSkillMiner: (
    caseId: number,
    action: "confirm" | "dismiss",
    sourceEpisodeId: string,
    skill?: SkillMinerSkill,
  ) =>
    post<SkillMinerDebugResult>("/skill-miner-review", {
      caseId,
      action,
      sourceEpisodeId,
      skill,
    }),
  compareSkill: (
    aiAgentId: number,
    skillDefinitionId: number,
    messages: { role: "user" | "assistant"; content: string }[],
  ) =>
    post<SkillCompareResult>("/skill-compare", {
      aiAgentId,
      skillDefinitionId,
      messages,
    }),
  export: (id: number) => requestBlob(base + `/export?id=${id}`),
};
