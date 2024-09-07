<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("sidebar.supportFile") }}</h2>
    </div>

    <div class="card-content">
      <div v-if="supportFileState === 'idle'">
        <p>
          This action will create a support file which can help us diagnose
          issues with RCade.
        </p>
        <p>
          The support file may contain sensitive information such as device IDs.
        </p>
        <p>Do not share this file with anyone other than RCade support.</p>
        <p>Would you like to proceed?</p>
      </div>

      <div v-else-if="supportFileState === 'creating'">
        <p>Generating support file</p>
        <p>This may take a while...</p>

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
      <div v-else-if="supportFileState === 'success'">
        <p>Support file created successfully!</p>
        <p>Click the button below to download the file.</p>
      </div>
    </div>

    <div class="card-action" v-if="supportFileState !== 'creating'">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
        tabindex="2"
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
import { mapActions, mapState } from "pinia";
import { useLayoutStore } from "@/stores/layout";
import { useAuthStore } from "@/stores/auth";
import { files } from "@/api";

export default {
  name: "supportFile",
  data() {
    return {
      supportFileState: "idle", // "creating", "idle", "success", "error"
      buttonLabel: this.$t("buttons.create"),
    };
  },
  computed: {
    ...mapState(useAuthStore, ["user"]),
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    createSupportFile() {
      this.supportFileState = "creating";
      fetch("/api/support")
        .then((response) => {
          if (response.ok) {
            this.supportFileState = "success";
            this.buttonLabel = this.$t("buttons.download");
          } else {
            this.supportFileState = "error";
            console.error(
              "Error generating support file:",
              response.statusText
            );
          }
        })
        .catch((error) => {
          this.supportFileState = "error";
          console.error("Error generating support file:", error);
        });
    },
    handleButton() {
      if (this.supportFileState === "idle") {
        this.createSupportFile();
      } else if (this.supportFileState === "success") {
        const fileURL = this.user.perm.admin
          ? "/files/rcade/share/supportFiles.tar.xz"
          : "/files/supportFiles.tar.xz";
        files.download(null, fileURL);
        this.closeHovers();
      } else if (this.supportFileState === "error") {
        this.createSupportFile();
      }
    },
  },
};
</script>
