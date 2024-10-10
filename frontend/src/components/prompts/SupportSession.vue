<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("sidebar.supportSession") }}</h2>
    </div>

    <div class="card-content">
      <div v-if="supportSessionState === 'idle'">
        <p>
          Would you like to start a support session? This will allow RCade
          Support to access your files and help you with any issues you may
          have.
        </p>
        <p>
          Do not share the session code with anyone other than RCade support.
        </p>
        <p>Would you like to proceed?</p>
      </div>

      <div v-else-if="supportSessionState === 'starting'">
        <p>Starting support session...</p>

        <loading>
          <h2 class="message delayed">
            <div class="spinner">
              <div class="bounce1"></div>
              <div class="bounce2"></div>
              <div class="bounce3"></div>
            </div>
          </h2>
        </loading>
      </div>
      <div v-else-if="supportSessionState === 'success'">
        <p>Support session created successfully!</p>
        <p>The support session can be accessed from the following URL:</p>
        <p>
          <span ref="proxyText">{{ sessionStatus.proxyURL }}</span>
        </p>
        <p>
          Session code:
          <strong>
            <span ref="sessionCode">
              {{ sessionStatus.sessionCode }}
            </span>
          </strong>
        </p>
        <button
          class="button button--block"
          @click="copyToClipboard(getSupportText())"
          :aria-label="$t('buttons.copyToClipboard')"
          :title="$t('buttons.copyToClipboard')"
        >
          {{ $t("buttons.copyToClipboard") }}
        </button>
      </div>
    </div>

    <div class="card-action" v-if="supportSessionState !== 'starting'">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
        tabindex="2"
        v-if="supportSessionState === 'idle'"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        id="focus-prompt"
        class="button button--flat"
        @click="handleButton"
        type="submit"
        :aria-label="buttonLabel"
        :title="buttonLabel"
        tabindex="1"
      >
        {{ buttonLabel }}
      </button>
    </div>
  </div>
</template>

<script>
import { mapActions } from "pinia";
import { useLayoutStore } from "@/stores/layout";
import { copy } from "@/utils/clipboard";

export default {
  name: "supportSession",
  data() {
    return {
      supportSessionState: "idle", // "starting", "idle", "success", "error"
      buttonLabel: this.$t("buttons.start"),
      sessionStatus: {
        proxyURL: "",
        timeStarted: "",
        sessionCode: "",
        started: false,
      },
    };
  },
  inject: ["$showError", "$showSuccess"],
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    startSupportSession() {
      this.supportSessionState = "starting";
      fetch("/api/support/start")
        .then((response) => {
          if (response.ok) {
            // Update state based on response status
            this.supportSessionState = "success";
            this.buttonLabel = this.$t("buttons.stop");
            return response.json(); // Return the parsed JSON for further processing
          } else {
            this.supportSessionState = "error";
            console.error(
              "Error starting support session:",
              response.statusText
            );
            throw new Error(response.statusText); // Throw error to be caught in .catch()
          }
        })
        .then((data) => {
          this.sessionStatus = data;
        })
        .catch((error) => {
          this.supportSessionState = "error";
          console.error("Error starting support session:", error);
        });
    },
    stopSupportSession() {
      fetch("/api/support/stop")
        .then((response) => {
          if (response.ok) {
            this.closeHovers();
          } else {
            throw new Error(response.statusText); // Throw error to be caught in .catch()
          }
        })
        .catch((error) => {
          this.supportSessionState = "error";
          console.error("Error stopping support session:", error);
        });
    },
    getSupportText() {
      const { proxyURL, sessionCode } = this.sessionStatus;
      return `Support URL: ${proxyURL}\nSession Code: ${sessionCode}`;
    },
    handleButton() {
      if (this.supportSessionState === "idle") {
        this.startSupportSession();
      } else if (this.supportSessionState === "success") {
        this.stopSupportSession();
      } else if (this.supportSessionState === "error") {
        this.startSupportSession();
      }
    },
    copyToClipboard: function (text) {
      copy(text).then(
        () => {
          // clipboard successfully set
          this.$showSuccess(this.$t("success.linkCopied"));
        },
        () => {
          // clipboard write failed
        }
      );
    },
  },
};
</script>
