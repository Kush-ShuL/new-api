/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

export type AvailabilityHours = 24 | 72 | 168

export type AvailabilityPoint = {
  ts: number
  request_count: number
  success_count: number
}

export type AvailabilityModel = {
  model_name: string
  icon?: string
  request_count: number
  series: AvailabilityPoint[]
}

export type AvailabilityData = {
  window_start: number
  window_end: number
  models: AvailabilityModel[]
}

export async function getModelAvailability(
  hours: AvailabilityHours,
  signal?: AbortSignal
): Promise<AvailabilityData> {
  const response = await api.get<{
    success: boolean
    message?: string
    data: AvailabilityData
  }>('/api/model-availability', { params: { hours }, signal })
  return requireServerSuccess(response.data).data
}
