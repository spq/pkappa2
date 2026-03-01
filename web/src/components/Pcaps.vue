<template>
  <div>
    <ToolBar>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn icon v-bind="props" @click="updatePcaps">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn icon v-bind="props" @click="openUploadDialog">
            <v-icon>mdi-upload</v-icon>
          </v-btn>
        </template>
        <span>Upload PCAP</span>
      </v-tooltip>
    </ToolBar>
    <v-card density="compact" variant="flat">
      <v-card-title>Processed PCAPs</v-card-title>
    </v-card>
    <v-data-table
      :headers="headers"
      :items="store.pcaps || []"
      :loading="store.pcaps === null"
      :items-per-page="20"
      :items-per-page-options="[20, 50, 100, -1]"
      hover
      density="compact"
    >
      <template #[`item.download`]="{ item }"
        ><v-btn
          variant="plain"
          density="compact"
          :href="`/api/download/pcap/${item.Filename}`"
          icon
        >
          <v-icon>mdi-download</v-icon>
        </v-btn></template
      >
      <!-- eslint-disable vue/no-v-for-template-key-on-child -->
      <template
        v-for="field of [
          'ParseTime',
          'PacketTimestampMin',
          'PacketTimestampMax',
        ]"
        #[`item.${field}`]="{ index, value }"
        ><span
          :key="`${field}/${index}`"
          :title="formatDateLong(new Date(value))"
          >{{ formatDate(new Date(value)) }}</span
        ></template
      >
      <template #[`item.Filesize`]="{ value }"
        ><span :title="`${value} Bytes`">{{
          prettyBytes(value, { maximumFractionDigits: 1, binary: true })
        }}</span></template
      >
    </v-data-table>
    <v-dialog
      v-model="uploadDialog"
      width="500"
      @keydown.enter="startUpload"
      @click:outside="closeUploadDialog"
    >
      <v-card>
        <v-card-title>Upload PCAP File</v-card-title>
        <v-card-text>
          <v-alert
            v-if="uploadError"
            type="error"
            variant="outlined"
            class="mb-4"
            :text="uploadError"
          />
          <v-file-input
            v-model="selectedFile"
            label="Select PCAP file to upload"
            accept=".pcap,.pcapng,application/vnd.tcpdump.pcap,application/octet-stream"
            show-size
            prepend-inner-icon="mdi-file"
            :disabled="uploading"
            required
            @change="onFileChange"
          />
          <v-text-field
            v-model="targetFilename"
            label="Target filename (.pcap or .pcapng)"
            :disabled="uploading"
            required
            clearable
          />
          <v-text-field
            v-model="pcapPassword"
            type="password"
            label="PCAP upload password (optional)"
            :disabled="uploading"
            prepend-inner-icon="mdi-lock"
          />
          <v-progress-linear
            v-if="uploading"
            :value="uploadProgress"
            height="6"
            color="primary"
            class="mt-2"
            striped
            rounded
          />
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn :disabled="uploading" variant="text" @click="closeUploadDialog"
            >Cancel</v-btn
          >
          <v-btn
            :disabled="!canUpload"
            :loading="uploading"
            variant="text"
            color="primary"
            @click="startUpload"
            >Upload</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-dialog>

    <!-- Batch upload area -->
    <v-card class="mt-4" variant="tonal">
      <v-card-title>Batch Upload</v-card-title>
      <v-card-text>
        <div
          class="batch-dropzone"
          :class="{ 'batch-dropzone--active': batchDragging }"
          @dragover.prevent="onBatchDragOver"
          @dragleave.prevent="onBatchDragLeave"
          @drop.prevent="onBatchDrop"
          @click="openBatchFilePicker"
        >
          <v-icon size="36">mdi-tray-arrow-up</v-icon>
          <div class="text-body-1 mt-2">
            Drop .pcap/.pcapng files here, or click to select
          </div>
          <div class="text-caption opacity-70">Multiple files supported</div>
          <input
            ref="batchFileInput"
            type="file"
            accept=".pcap,.pcapng,application/vnd.tcpdump.pcap,application/octet-stream"
            multiple
            class="d-none"
            @change="onBatchFilePicked"
          />
        </div>

        <div v-if="queuedFiles.length > 0" class="mt-4">
          <div class="text-subtitle-2 mb-2">
            Selected files ({{ queuedFiles.length }})
          </div>
          <v-table density="compact">
            <tbody>
              <tr v-for="f in queuedFiles" :key="f.name + ':' + f.size">
                <td class="py-2">{{ f.name }}</td>
                <td class="py-2 text-no-wrap">
                  {{
                    prettyBytes(f.size, {
                      maximumFractionDigits: 1,
                      binary: true,
                    })
                  }}
                </td>
                <td class="py-2 text-right w0">
                  <v-btn
                    icon
                    density="comfortable"
                    variant="text"
                    @click="removeQueuedFile(f)"
                  >
                    <v-icon>mdi-close</v-icon>
                  </v-btn>
                </td>
              </tr>
            </tbody>
          </v-table>

          <div class="d-flex align-center ga-2 mt-3">
            <v-text-field
              v-model="pcapPassword"
              label="PCAP upload password (optional)"
              type="password"
              density="comfortable"
              prepend-inner-icon="mdi-lock"
              class="flex-grow-1"
            />
            <v-btn
              :disabled="batchUploading || queuedFiles.length === 0"
              variant="text"
              @click="clearQueued"
            >
              Clear
            </v-btn>
            <v-btn
              color="primary"
              :loading="batchUploading"
              :disabled="queuedFiles.length === 0"
              @click="uploadBatch"
            >
              Upload {{ queuedFiles.length }} file(s)
            </v-btn>
          </div>

          <v-progress-linear
            v-if="batchUploading"
            :model-value="batchProgress"
            height="6"
            color="primary"
            class="mt-2"
            striped
            rounded
          />
        </div>
      </v-card-text>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, ref } from "vue";
import { useRootStore } from "@/stores";
import { EventBus } from "./EventBus";
import { formatDate, formatDateLong } from "@/filters";
import prettyBytes from "pretty-bytes";

const store = useRootStore();
const headers = [
  {
    title: "File Name",
    value: "Filename",
  },
  {
    title: "First Packet Time",
    value: "PacketTimestampMin",
  },
  {
    title: "Last Packet Time",
    value: "PacketTimestampMax",
  },
  {
    title: "Packet Count",
    value: "PacketCount",
  },
  {
    title: "File Size",
    value: "Filesize",
  },
  {
    title: "Parse Time",
    value: "ParseTime",
    align: "end",
    class: "pr-0",
    cellClass: "pr-0",
  },
  {
    title: "",
    value: "download",
    sortable: false,
    class: ["px-0", "w0"],
    cellClass: ["px-0", "w0"],
  },
] as const;
const uploadDialog = ref(false);
// batch upload states
const batchDragging = ref(false);
const batchFileInput = ref<HTMLInputElement | null>(null);
const queuedFiles = ref<File[]>([]);
const batchUploading = ref(false);
const uploadedCount = ref(0);
const batchProgress = ref(0);
const selectedFile = ref<File | null>(null);
const targetFilename = ref<string>("");
const pcapPassword = ref<string>("");
const uploading = ref(false);
const uploadProgress = ref(0);
const uploadError = ref<string | null>(null);

function updatePcaps() {
  store.updatePcaps().catch((err: Error) => {
    EventBus.emit("showError", `Failed to update pcaps: ${err.message}`);
  });
}

onMounted(() => {
  updatePcaps();
});

const canUpload = computed(() => {
  if (!selectedFile.value) return false;
  const name = targetFilename.value.trim() || selectedFile.value.name;
  return /\.(pcap|pcapng)$/i.test(name);
});

function openUploadDialog() {
  uploadDialog.value = true;
  uploadError.value = null;
  uploadProgress.value = 0;
}

function closeUploadDialog(force = false) {
  if (uploading.value && !force) return;
  uploadDialog.value = false;
  selectedFile.value = null;
  targetFilename.value = "";
  uploadProgress.value = 0;
  uploadError.value = null;
}

function onFileChange() {
  if (selectedFile.value && !targetFilename.value) {
    targetFilename.value = selectedFile.value.name;
  }
}

function startUpload() {
  if (!selectedFile.value) {
    uploadError.value = "Please select a file to upload.";
    return;
  }

  const filename = (targetFilename.value || selectedFile.value.name).trim();
  if (!filename.endsWith(".pcap") && !filename.endsWith(".pcapng")) {
    uploadError.value = "Target filename must end with .pcap or .pcapng.";
    return;
  }

  uploading.value = true;
  uploadError.value = null;
  uploadProgress.value = 0;
  store
    .uploadPcap(
      selectedFile.value,
      filename,
      pcapPassword.value || undefined,
      (progress) => {
        uploadProgress.value = progress;
      },
    )
    .then(() => {
      EventBus.emit("showMessage", `Uploaded ${filename} successfully.`);
      uploading.value = false;
      closeUploadDialog(true);
    })
    .catch((err: Error) => {
      uploadError.value = `Upload failed: ${err.message}`;
      EventBus.emit("showError", `Failed to upload pcap: ${uploadError.value}`);
    })
    .finally(() => {
      uploading.value = false;
      uploadProgress.value = 0;
    });
}

function onBatchDragOver(e: DragEvent) {
  if (!e.dataTransfer) return;
  const types = e.dataTransfer.types;
  if (types && Array.from(types).includes("Files")) {
    e.dataTransfer.dropEffect = "copy";
    batchDragging.value = true;
  }
}

function onBatchDragLeave() {
  batchDragging.value = false;
}

function onBatchDrop(e: DragEvent) {
  batchDragging.value = false;
  const files = Array.from(e.dataTransfer?.files || []);
  addFilesToQueue(files);
}
function openBatchFilePicker() {
  batchFileInput.value?.click();
}
function onBatchFilePicked(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const files = Array.from(input.files || []);
  addFilesToQueue(files);
  if (input) input.value = "";
}
function addFilesToQueue(files: File[]) {
  const accepted: File[] = [];
  const rejected: string[] = [];
  for (const f of files) {
    if (f.name.endsWith(".pcap") || f.name.endsWith(".pcapng"))
      accepted.push(f);
    else rejected.push(f.name);
  }
  if (rejected.length) {
    EventBus.emit(
      "showError",
      `Unsupported files skipped: ${rejected.slice(0, 5).join(", ")}${
        rejected.length > 5 ? ` and ${rejected.length - 5} more` : ""
      }`,
    );
  }
  if (accepted.length === 0) return;
  const existing = new Set(queuedFiles.value.map((f) => `${f.name}:${f.size}`));
  for (const f of accepted) {
    const key = `${f.name}:${f.size}`;
    if (!existing.has(key)) queuedFiles.value.push(f);
  }
}
function removeQueuedFile(file: File) {
  queuedFiles.value = queuedFiles.value.filter(
    (f) => f.name !== file.name || f.size !== file.size,
  );
}
function clearQueued() {
  queuedFiles.value = [];
}
async function uploadBatch() {
  if (queuedFiles.value.length === 0 || batchUploading.value) return;
  batchUploading.value = true;
  uploadedCount.value = 0;
  batchProgress.value = 0;
  const files = [...queuedFiles.value];
  let encounteredError = false;
  try {
    for (const f of files) {
      try {
        await store.uploadPcap(
          f,
          f.name,
          pcapPassword.value || undefined,
          (p) => {
            batchProgress.value = Math.round(
              ((uploadedCount.value + p / 100) / files.length) * 100,
            );
          },
        );
        uploadedCount.value++;
        batchProgress.value = Math.round(
          (uploadedCount.value / files.length) * 100,
        );
        queuedFiles.value = queuedFiles.value.filter(
          (existing) => existing !== f,
        );
      } catch (err: unknown) {
        const message =
          typeof err === "object" && err !== null && "message" in err
            ? String((err as { message: unknown }).message)
            : String(err);
        EventBus.emit("showError", `Failed to upload ${f.name}: ${message}`);
        encounteredError = true;
      }
    }
  } finally {
    const uploaded = uploadedCount.value;
    batchUploading.value = false;
    if (uploaded > 0) {
      if (!encounteredError && queuedFiles.value.length === 0) {
        batchProgress.value = 100;
      }
      await store.updateStatus();
      EventBus.emit("showMessage", `Uploaded ${uploaded} file(s).`);
    }
    if (!encounteredError && queuedFiles.value.length === 0) {
      window.setTimeout(() => {
        batchProgress.value = 0;
      }, 300);
    } else {
      batchProgress.value = Math.round((uploaded / (files.length || 1)) * 100);
    }
    uploadedCount.value = 0;
  }
}
</script>

<style scoped>
.w0 {
  width: 0;
}
.batch-dropzone {
  border: 2px dashed rgba(25, 118, 210, 0.6);
  border-radius: 8px;
  padding: 24px;
  text-align: center;
  cursor: pointer;
  transition:
    background-color 120ms ease,
    box-shadow 120ms ease,
    border-color 120ms ease;
}
.batch-dropzone--active {
  background-color: rgba(25, 118, 210, 0.06);
  border-color: rgba(25, 118, 210, 0.9);
}
.opacity-70 {
  opacity: 0.7;
}
</style>
