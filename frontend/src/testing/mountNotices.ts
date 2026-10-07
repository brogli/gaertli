import type { DOMWrapper } from '@vue/test-utils'
import NoticeArea from '@/components/NoticeArea.vue'
import { mountInBody } from '@/testing/mountInBody'

// mountNotices mounts the notice area in the document body and returns
// body for queries into the toasts it teleports.
export async function mountNotices(): Promise<DOMWrapper<Element>> {
  return (await mountInBody(NoticeArea)).body
}
