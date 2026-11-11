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
import { expect, it } from 'vitest'

import { buildUsageSeries } from '../lib/usage-series'
import type { ModelHistorySeries } from '../types'

const history: ModelHistorySeries = {
  window_start: 3600,
  window_end: 10800,
  bucket_seconds: 3600,
  buckets: 3,
  models: [
    { name: 'a', vendor: 'A', total: 120 },
    { name: 'b', vendor: 'B', total: 50 },
  ],
  points: [
    {
      ts: '1970-01-01T01:00:00Z',
      label: '01:00',
      model: 'a',
      vendor: 'A',
      tokens: 100,
      requests: 2,
    },
    {
      ts: '1970-01-01T01:00:00Z',
      label: '01:00',
      model: 'b',
      vendor: 'B',
      tokens: 50,
      requests: 3,
    },
    {
      ts: '1970-01-01T03:00:00Z',
      label: '03:00',
      model: 'a',
      vendor: 'A',
      tokens: 20,
      requests: 4,
    },
  ],
}

it('keeps model request series separate and fills an idle hour with zero', () => {
  expect(buildUsageSeries(history, 'requests')).toEqual([
    { ts: 3600, model: 'a', value: 2 },
    { ts: 3600, model: 'b', value: 3 },
    { ts: 7200, model: 'a', value: 0 },
    { ts: 7200, model: 'b', value: 0 },
    { ts: 10800, model: 'a', value: 4 },
    { ts: 10800, model: 'b', value: 0 },
  ])
})

it('filters by the canonical model ID and keeps token and request metrics separate', () => {
  expect(buildUsageSeries(history, 'tokens', new Set(['b']))).toEqual([
    { ts: 3600, model: 'b', value: 50 },
    { ts: 7200, model: 'b', value: 0 },
    { ts: 10800, model: 'b', value: 0 },
  ])
})
