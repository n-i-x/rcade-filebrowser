<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("sidebar.sysInfo") }}</h2>
    </div>

    <div class="card-content">
      <p v-html="formattedOutput"></p>
      <p>Auto-refreshing in {{ countdown }}s</p>
    </div>

    <div class="card-action">
      <button
        id="focus-prompt"
        type="submit"
        @click="closeHovers"
        class="button button--flat"
        :aria-label="$t('buttons.ok')"
        :title="$t('buttons.ok')"
        tabindex="1"
      >
        {{ $t("buttons.ok") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mapActions } from "pinia";
import { useLayoutStore } from "@/stores/layout";

export default {
  name: "systemInfo",
  data() {
    return {
      systemInfo: {
        returnCode: 0,
        output: "",
      },
      countdown: 10,
    };
  },
  computed: {
    formattedOutput() {
      // Replace \n with <br> for HTML rendering
      return this.systemInfo.output.replace(/\n/g, "<br>");
    },
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    fetchSystemInfo() {
      fetch("/api/sysinfo")
        .then((response) => response.json())
        .then((data) => {
          this.systemInfo = data;
          this.countdown = 10;
        })
        .catch((error) => {
          console.error("Error fetching system info:", error);
        });
    },
    startCountdown() {
      // Decrease the countdown every second
      this.countdownInterval = setInterval(() => {
        if (this.countdown > 0) {
          this.countdown--;
        }
      }, 1000);
    },
  },
  mounted() {
    this.fetchSystemInfo();
    this.startCountdown();
    // Fetch data every 10 seconds and reset countdown
    this.fetchInterval = setInterval(() => {
      this.fetchSystemInfo();
      this.countdown = 10;
    }, 10000);
  },
  beforeUnmount() {
    // Clear interval to prevent memory leaks
    clearInterval(this.interval);
    clearInterval(this.countdownInterval);
  },
};
</script>
