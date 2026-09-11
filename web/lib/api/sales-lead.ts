import { request } from "@/lib/api/client";
import type { LeadStatus, LeadTag } from "@/lib/generated/enums";
import type { MemoryEntry } from "@/lib/api/conversation-memory";

export type LeadData = {
  title: string;
  product: string;
  quantity: string;
  destination: string;
  budget: string;
  timeline: string;
  contactName: string;
  company: string;
  email: string;
  phone: string;
  needs: string;
  autoTags: LeadTag[];
};
export type SalesLead = {
  id: number;
  conversationId: number;
  customerId: number;
  channelId: number;
  customerName: string;
  data: LeadData;
  proposal?: LeadData;
  confirmed: boolean;
  withdrawn: boolean;
  sourceValid: boolean;
  status: LeadStatus;
  ownerId: number;
  ownerName: string;
  customTags: string[];
  note: string;
  followUpAt?: string;
  nextAction: string;
  lastFollowUpAt?: string;
  revision: number;
  updatedAt: string;
  sources: MemoryEntry["sources"];
  events: { id: number; kind: string; actorId: number; createdAt: string; followUp?: LeadFollowUp }[];
};
export type LeadFollowUp = {
  revision: number;
  status: LeadStatus;
  ownerId: number;
  result: string;
  nextAction: string;
  followUpAt: string | null;
};
export function recordLeadFollowUp(id: number, payload: LeadFollowUp) {
  return request<void>(`/api/dashboard/conversation/sales_leads/${id}/follow_up`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}
export type LeadUpdate = {
  revision: number;
  data: LeadData;
  status: LeadStatus;
  ownerId: number;
  customTags: string[];
  note: string;
  followUpAt: string | null;
  acceptProposal: boolean;
};
export function fetchSalesLeads(
  query: Record<string, string | number | undefined> = {},
) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query))
    if (value !== undefined && value !== "") params.set(key, String(value));
  return request<{
    results: SalesLead[];
    page: { page: number; limit: number; total: number };
  }>(`/api/dashboard/conversation/sales_leads?${params}`);
}
export function fetchSalesLead(id: number) {
  return request<SalesLead>(`/api/dashboard/conversation/sales_leads/${id}`);
}
export function updateSalesLead(id: number, payload: LeadUpdate) {
  return request<void>(`/api/dashboard/conversation/sales_leads/${id}/update`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}
