<template>
  <Teleport to="body">
    <Transition name="popup-fade">
      <div
        v-if="displayedAnnouncement"
        class="modal-overlay announcement-popup-overlay z-[120]"
      >
        <div
          class="modal-content w-full max-w-[680px] overflow-hidden"
          @click.stop
        >
          <div class="modal-header block px-8 py-6">
            <div>
              <!-- Icon and badge -->
              <div class="mb-3 flex items-center gap-2">
                <div class="flex h-10 w-10 items-center justify-center rounded bg-[var(--nx-accent)] text-white">
                  <Icon name="bell" size="md" />
                </div>
                <span class="badge badge-primary">
                  <span class="relative flex h-2 w-2">
                    <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--nx-accent)] opacity-75"></span>
                    <span class="relative inline-flex h-2 w-2 rounded-full bg-[var(--nx-accent)]"></span>
                  </span>
                  {{ t('announcements.unread') }}
                </span>
              </div>

              <!-- Title -->
              <h2 class="mb-2 text-2xl font-semibold leading-tight text-[var(--nx-text)]">
                {{ displayedAnnouncement.title }}
              </h2>

              <!-- Time -->
              <div class="flex items-center gap-1.5 text-sm text-[var(--nx-muted)]">
                <Icon name="clock" size="sm" />
                <time>{{ formatRelativeWithDateTime(displayedAnnouncement.created_at) }}</time>
              </div>
            </div>
          </div>

          <!-- Body -->
          <div class="max-h-[50vh] overflow-y-auto bg-[var(--nx-surface)] px-8 py-8">
            <div class="relative">
              <div class="absolute bottom-0 left-0 top-0 w-1 rounded-full bg-[var(--nx-accent)]"></div>
              <div class="pl-6">
                <div
                  class="markdown-body max-w-none"
                  v-html="renderedContent"
                ></div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="modal-footer px-8 py-5">
            <div class="flex items-center justify-end">
              <button
                @click="handleDismiss"
                data-testid="announcement-popup-dismiss"
                class="btn btn-primary"
              >
                <span class="flex items-center gap-2">
                  <Icon :name="preview ? 'x' : 'check'" size="sm" :stroke-width="2" />
                  {{ preview ? t('common.close') : t('announcements.markRead') }}
                </span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeWithDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import type { Announcement, UserAnnouncement } from '@/types'

type PreviewAnnouncement = Pick<Announcement | UserAnnouncement, 'title' | 'content' | 'created_at'>

const props = withDefaults(defineProps<{
  announcement?: PreviewAnnouncement | null
  preview?: boolean
}>(), {
  announcement: null,
  preview: false,
})

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const announcementStore = useAnnouncementStore()
const displayedAnnouncement = computed(() => (
  props.preview ? props.announcement : announcementStore.currentPopup
))

marked.setOptions({
  breaks: true,
  gfm: true,
})

const renderedContent = computed(() => {
  const content = displayedAnnouncement.value?.content
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})

function handleDismiss() {
  if (props.preview) {
    emit('close')
    return
  }
  announcementStore.dismissPopup()
}

// Manage body overflow — only set, never unset (bell component handles restore)
watch(
  displayedAnnouncement,
  (popup) => {
    if (popup) {
      document.body.style.overflow = 'hidden'
    } else if (props.preview) {
      document.body.style.overflow = ''
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (props.preview) {
    document.body.style.overflow = ''
  }
})
</script>

<style scoped>
.popup-fade-enter-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.popup-fade-leave-active {
  transition: all 0.2s cubic-bezier(0.4, 0, 1, 1);
}

.popup-fade-enter-from,
.popup-fade-leave-to {
  opacity: 0;
}

.popup-fade-enter-from > div {
  transform: scale(0.94) translateY(-12px);
  opacity: 0;
}

.popup-fade-leave-to > div {
  transform: scale(0.96) translateY(-8px);
  opacity: 0;
}

.announcement-popup-overlay {
  align-items: flex-start;
  overflow-y: auto;
  padding-top: 8vh;
}

</style>
