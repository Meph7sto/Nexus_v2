import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";

import Toggle from "../Toggle.vue";

describe("Toggle", () => {
  it("uses the Nexus visual tokens and updates its model value", async () => {
    const wrapper = mount(Toggle, {
      props: { modelValue: false },
    });

    const button = wrapper.get("button");
    expect(button.attributes("role")).toBe("switch");
    expect(button.classes()).toContain("focus:ring-[rgba(255,86,0,0.22)]");
    expect(button.classes()).toContain("bg-[var(--nx-border-strong)]");
    expect(wrapper.get("span").classes()).toContain("bg-[var(--nx-surface)]");

    await button.trigger("click");
    expect(wrapper.emitted("update:modelValue")).toEqual([[true]]);

    await wrapper.setProps({ modelValue: true });
    expect(button.classes()).toContain("bg-[var(--nx-accent)]");
  });
});
