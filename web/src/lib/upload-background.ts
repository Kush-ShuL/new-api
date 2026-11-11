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
import { customAlphabet } from 'nanoid'

import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

const createUploadId = customAlphabet('0123456789abcdef', 36)

/** Small sequential parts also work behind proxies with request size limits. */
export async function uploadBackground(
  file: File,
  endpoint = '/api/option/appearance/background'
): Promise<string> {
  const uploadId = createUploadId()
  let url = ''
  for (let offset = 0; offset < file.size; offset += 8 * 1024 * 1024) {
    const data = new FormData()
    data.append(
      'image',
      file.slice(offset, offset + 8 * 1024 * 1024),
      file.name
    )
    const response = await api.post(endpoint, data, {
      timeout: 180_000,
      params: { upload_id: uploadId, offset, total: file.size },
    })
    url = requireServerSuccess(response.data).data.url as string
  }
  return url
}
