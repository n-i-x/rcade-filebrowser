<template>
  <VueFinalModal
    class="vfm-modal"
    overlay-transition="vfm-fade"
    content-transition="vfm-fade"
    @closed="layoutStore.closeHovers"
    :focus-trap="{
      initialFocus: '#focus-prompt',
      fallbackFocus: 'div.vfm__content',
    }"
    :click-to-close="clickToClose"
    :esc-to-close="clickToClose"
  >
    <slot />
  </VueFinalModal>
</template>

<script setup lang="ts">
import { VueFinalModal } from "vue-final-modal";
import { useLayoutStore } from "@/stores/layout";
import { defineProps } from "vue";

const props = defineProps({
  clickToClose: {
    type: Boolean,
    default: true,
  },
});

const clickToClose = props.clickToClose;
const layoutStore = useLayoutStore();
</script>
