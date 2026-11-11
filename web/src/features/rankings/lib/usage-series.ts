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
import type { ModelHistorySeries } from '../types'

export type UsageMetric = 'requests' | 'tokens'

export function buildUsageSeries(
  history: ModelHistorySeries,
  metric: UsageMetric,
  models?: Set<string>
): { ts: number; model: string; value: number }[] {
  const names = history.models
    .map((model) => model.name)
    .filter((name) => !models || models.has(name))
  const values = new Map<string, number>()
  const timestamps = new Set<number>()
  for (const point of history.points) {
    const ts = Date.parse(point.ts) / 1000
    timestamps.add(ts)
    values.set(`${ts}:${point.model}`, point[metric] ?? 0)
  }
  if (
    history.window_start != null &&
    history.window_end != null &&
    history.bucket_seconds
  ) {
    const size = history.bucket_seconds
    const start = Math.floor(history.window_start / size) * size
    const end = Math.floor(history.window_end / size) * size
    for (let ts = start; ts <= end; ts += size) {
      timestamps.add(ts)
    }
  }
  return [...timestamps]
    .sort((a, b) => a - b)
    .flatMap((ts) =>
      names.map((model) => ({
        ts,
        model,
        value: values.get(`${ts}:${model}`) ?? 0,
      }))
    )
}
