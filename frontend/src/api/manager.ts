/**
 * Manager API endpoints
 * Department-manager scoped endpoints (JWT + manager role required).
 * The backend re-validates department scope for every call; IDs in the
 * URL are never trusted alone.
 */

import { apiClient } from './client'
import type {
  UserSubscription,
  SubscriptionProgress,
  PaginatedResponse
} from '@/types'

export interface ManagerMemberSubscription extends UserSubscription {
  subscription_id?: number
}

export interface ManagerMember {
  id: number
  email: string
  username: string
  role: string
  status: string
  primary_dept_id?: number | null
  subscriptions: ManagerMemberSubscription[]
}

/**
 * List department members (with existing subscription records) in the
 * manager's scope.
 */
export async function listMembers(
  page: number = 1,
  pageSize: number = 20,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<ManagerMember>> {
  const { data } = await apiClient.get<PaginatedResponse<ManagerMember>>(
    '/manager/members',
    {
      params: { page, page_size: pageSize },
      signal: options?.signal
    }
  )
  return data
}

/**
 * Get usage progress for one member subscription.
 */
export async function getSubscriptionProgress(
  id: number,
  options?: { signal?: AbortSignal }
): Promise<SubscriptionProgress> {
  const { data } = await apiClient.get<SubscriptionProgress>(
    `/manager/subscriptions/${id}/progress`,
    { signal: options?.signal }
  )
  return data
}

/**
 * Reset daily, weekly, and/or monthly usage windows for one member
 * subscription.
 */
export async function resetQuota(
  id: number,
  options: { daily: boolean; weekly: boolean; monthly: boolean }
): Promise<UserSubscription> {
  const { data } = await apiClient.post<UserSubscription>(
    `/manager/subscriptions/${id}/reset-quota`,
    options
  )
  return data
}

export const managerAPI = {
  listMembers,
  getSubscriptionProgress,
  resetQuota
}

export default managerAPI
