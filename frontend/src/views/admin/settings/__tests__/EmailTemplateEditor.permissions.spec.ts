import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";

import EmailTemplateEditor from "../EmailTemplateEditor.vue";

const authStore = vi.hoisted(() => ({
  canAdmin: vi.fn(),
}));

const settingsAPI = vi.hoisted(() => ({
  getEmailTemplates: vi.fn(),
  getEmailTemplate: vi.fn(),
  updateEmailTemplate: vi.fn(),
  previewEmailTemplate: vi.fn(),
  restoreOfficialEmailTemplate: vi.fn(),
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: () => authStore,
}));

vi.mock("@/stores", () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}));

vi.mock("@/api", () => ({
  adminAPI: {
    settings: settingsAPI,
  },
}));

vi.mock("@/utils/apiError", () => ({
  extractApiErrorMessage: () => "error",
}));

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => key,
    locale: { value: "en" },
  }),
}));

describe("EmailTemplateEditor permissions", () => {
  beforeEach(() => {
    authStore.canAdmin.mockReset();
    authStore.canAdmin.mockReturnValue(false);
    settingsAPI.getEmailTemplates.mockResolvedValue({
      events: ["auth.verify_code"],
      locales: ["en"],
      placeholders: [],
    });
    settingsAPI.getEmailTemplate.mockResolvedValue({
      subject: "Verify your email",
      html: "<p>Code: {{verification_code}}</p>",
      is_custom: false,
      placeholders: [],
    });
  });

  it("does not render preview, restore, or save without the matching settings action", async () => {
    const wrapper = mount(EmailTemplateEditor);
    await flushPromises();

    const buttonText = wrapper.findAll("button").map((button) => button.text());

    expect(buttonText.some((text) => text.includes("admin.settings.emailTemplates.preview"))).toBe(false);
    expect(buttonText.some((text) => text.includes("admin.settings.emailTemplates.restoreOfficial"))).toBe(false);
    expect(buttonText.some((text) => text.includes("admin.settings.emailTemplates.save"))).toBe(false);
  });
});
