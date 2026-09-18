/**
 * Admin API endpoints for assigning department scopes to managers.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface ManagerScopeDepartment {
  dept_id: number
  parent_id: number
  name: string
  group_name: string
  path: string[]
  is_active: boolean
  synced: boolean
}

export interface ManagerScopeManager {
  id: number
  email: string
  username: string
  role: string
  status: string
  primary_dept_id?: number | null
}

export async function listDepartments(): Promise<ManagerScopeDepartment[]> {
  const { data } = await apiClient.get<ManagerScopeDepartment[]>('/admin/manager-scopes/departments')
  return data
}

export async function syncDepartments(): Promise<number> {
  const { data } = await apiClient.post<{ synced: number }>('/admin/manager-scopes/departments/sync')
  return data.synced
}

export async function listManagers(
  page = 1,
  pageSize = 20,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<ManagerScopeManager>> {
  const { data } = await apiClient.get<PaginatedResponse<ManagerScopeManager>>(
    '/admin/manager-scopes/managers',
    {
      params: { page, page_size: pageSize },
      signal: options?.signal
    }
  )
  return data
}

export async function getManagerDepartments(managerId: number): Promise<ManagerScopeDepartment[]> {
  const { data } = await apiClient.get<ManagerScopeDepartment[]>(
    `/admin/manager-scopes/managers/${managerId}/departments`
  )
  return data
}

export async function updateManagerDepartments(
  managerId: number,
  departmentIds: number[]
): Promise<ManagerScopeDepartment[]> {
  const { data } = await apiClient.put<ManagerScopeDepartment[]>(
    `/admin/manager-scopes/managers/${managerId}/departments`,
    { department_ids: departmentIds }
  )
  return data
}

const managerScopesAPI = {
  listDepartments,
  syncDepartments,
  listManagers,
  getManagerDepartments,
  updateManagerDepartments
}

export default managerScopesAPI
