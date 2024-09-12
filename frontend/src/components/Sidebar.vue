<template>
  <div v-show="active" @click="closeHovers" class="overlay"></div>
  <nav :class="{ active }">
    <template v-if="isLoggedIn">
      <button
        class="action"
        @click="toRoot"
        :aria-label="$t('sidebar.myFiles')"
        :title="$t('sidebar.myFiles')"
      >
        <i class="material-icons">folder</i>
        <span>{{ $t("sidebar.myFiles") }}</span>
      </button>

      <div v-if="user.perm.create">
        <button
          @click="showHover('newDir')"
          class="action"
          :aria-label="$t('sidebar.newFolder')"
          :title="$t('sidebar.newFolder')"
        >
          <i class="material-icons">create_new_folder</i>
          <span>{{ $t("sidebar.newFolder") }}</span>
        </button>

        <button
          @click="showHover('newFile')"
          class="action"
          :aria-label="$t('sidebar.newFile')"
          :title="$t('sidebar.newFile')"
        >
          <i class="material-icons">note_add</i>
          <span>{{ $t("sidebar.newFile") }}</span>
        </button>
      </div>

      <div v-if="hasPixelcade">
        <button
          class="action"
          @click="toPixelcade"
          :aria-label="$t('sidebar.pixelcade')"
          :title="$t('sidebar.pixelcade')"
        >
          <i class="material-icons">apps</i>
          <span>{{ $t("sidebar.pixelcade") }}</span>
        </button>
      </div>
      <div>
        <button
          class="action"
          @click="toLogs"
          :aria-label="$t('sidebar.logs')"
          :title="$t('sidebar.logs')"
        >
          <i class="material-icons">receipt_long</i>
          <span>{{ $t("sidebar.logs") }}</span>
        </button>
        <button
          @click="showHover('supportFile')"
          class="action"
          :aria-label="$t('sidebar.supportFile')"
          :title="$t('sidebar.supportFile')"
        >
          <i class="material-icons">support_agent</i>
          <span>{{ $t("sidebar.supportFile") }}</span>
        </button>
        <button
          @click="showHover('systemInfo')"
          class="action"
          :aria-label="$t('sidebar.sysInfo')"
          :title="$t('sidebar.sysInfo')"
        >
          <i class="material-icons">info</i>
          <span>{{ $t("sidebar.sysInfo") }}</span>
        </button>
      </div>
      <div>
        <button
          class="action"
          @click="toSettings"
          :aria-label="$t('sidebar.settings')"
          :title="$t('sidebar.settings')"
        >
          <i class="material-icons">settings_applications</i>
          <span>{{ $t("sidebar.settings") }}</span>
        </button>

        <button
          v-if="canLogout"
          @click="logout"
          class="action"
          id="logout"
          :aria-label="$t('sidebar.logout')"
          :title="$t('sidebar.logout')"
        >
          <i class="material-icons">exit_to_app</i>
          <span>{{ $t("sidebar.logout") }}</span>
        </button>
      </div>
    </template>
    <template v-else>
      <router-link
        class="action"
        to="/login"
        :aria-label="$t('sidebar.login')"
        :title="$t('sidebar.login')"
      >
        <i class="material-icons">exit_to_app</i>
        <span>{{ $t("sidebar.login") }}</span>
      </router-link>

      <router-link
        v-if="signup"
        class="action"
        to="/login"
        :aria-label="$t('sidebar.signup')"
        :title="$t('sidebar.signup')"
      >
        <i class="material-icons">person_add</i>
        <span>{{ $t("sidebar.signup") }}</span>
      </router-link>
    </template>

    <div
      class="credits"
      v-if="isFiles && !disableUsedPercentage"
      style="width: 90%; margin: 2em 2.5em 3em 2.5em"
    >
      <progress-bar :val="usage.usedPercentage" size="small"></progress-bar>
      <br />
      {{ usage.used }} of {{ usage.total }} used
    </div>

    <p class="credits">
      <span>
        <span>RCade Web File Browser</span>
        <span> {{ version }}</span>
      </span>
      <span>
        <a @click="help">{{ $t("sidebar.help") }}</a>
      </span>
    </p>
  </nav>
</template>

<script>
import { reactive } from "vue";
import { mapActions, mapState } from "pinia";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import * as auth from "@/utils/auth";
import {
  version,
  signup,
  disableExternal,
  disableUsedPercentage,
  noAuth,
  loginPage,
} from "@/utils/constants";
import { files as api } from "@/api";
import ProgressBar from "@/components/ProgressBar.vue";
import prettyBytes from "pretty-bytes";

const USAGE_DEFAULT = { used: "0 B", total: "0 B", usedPercentage: 0 };

export default {
  name: "sidebar",
  setup() {
    const usage = reactive(USAGE_DEFAULT);
    return { usage };
  },
  components: {
    ProgressBar,
  },
  data() {
    return {
      hasPixelcade: false, // Initialize the variable
    };
  },
  inject: ["$showError"],
  computed: {
    ...mapState(useAuthStore, ["user", "isLoggedIn"]),
    ...mapState(useFileStore, ["isFiles", "reload"]),
    ...mapState(useLayoutStore, ["currentPromptName"]),
    active() {
      return this.currentPromptName === "sidebar";
    },
    signup: () => signup,
    version: () => version,
    disableExternal: () => disableExternal,
    disableUsedPercentage: () => disableUsedPercentage,
    canLogout: () => !noAuth && loginPage,
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers", "showHover"]),
    async fetchUsage() {
      let path = this.$route.path.endsWith("/")
        ? this.$route.path
        : this.$route.path + "/";
      let usageStats = USAGE_DEFAULT;
      if (this.disableUsedPercentage) {
        return Object.assign(this.usage, usageStats);
      }
      try {
        let usage = await api.usage(path);
        usageStats = {
          used: prettyBytes(usage.used, { binary: true }),
          total: prettyBytes(usage.total, { binary: true }),
          usedPercentage: Math.round((usage.used / usage.total) * 100),
        };
      } catch (error) {
        this.$showError(error);
      }
      return Object.assign(this.usage, usageStats);
    },
    checkPixelcade() {
      const currentHost = window.location.hostname; // Get the current hostname
      const url = `http://${currentHost}:8080/api/device/info`;

      // Perform the fetch request
      fetch(url, { mode: "no-cors" })
        .then(() => {
          // If no error, port is likely open
          this.hasPixelcade = true;
        })
        .catch(() => {
          // If fetch fails, port is closed or inaccessible
          this.hasPixelcade = false;
        });
    },
    toLogs() {
      this.$router.push({ path: "/files/virtual/logs" });
      this.closeHovers();
    },
    toPixelcade() {
      const currentHost = window.location.hostname; // Get the current hostname
      const url = `http://${currentHost}:8080`; // Change only the port to 8080
      window.open(url, "_blank"); // Open the new URL in a new tab
      this.closeHovers();
    },
    toRoot() {
      this.$router.push({ path: "/files" });
      this.closeHovers();
    },
    toSettings() {
      this.$router.push({ path: "/settings" });
      this.closeHovers();
    },
    help() {
      this.showHover("help");
    },
    logout: auth.logout,
  },
  watch: {
    isFiles(newValue) {
      newValue && this.fetchUsage();
    },
  },
  mounted() {
    // Check for Pixelcade when the component is mounted
    this.checkPixelcade();
  },
};
</script>
