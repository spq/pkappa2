import { defineStore } from "pinia";
import axios from "axios";
import APIClient from "@/apiClient";
import { StreamData } from "@/apiClient";

interface State {
  id: number | null;
  running: boolean;
  error: string | null;
  stream: StreamData | null;
}

export const useStreamStore = defineStore("stream", {
  state: (): State => ({
    id: null,
    running: false,
    error: null,
    stream: null,
  }),
  actions: {
    async fetchStream(id: number, converter: string) {
      this.id = id;
      this.running = true;
      this.error = null;
      this.stream = null;
      return APIClient.getStream(id, converter)
        .then((data) => {
          this.id = id;
          this.error = null;
          this.stream = data;
          this.running = false;
        })
        .catch((err) => {
          if (
            axios.isAxiosError<string, unknown>(err) &&
            err.response !== undefined
          ) {
            this.id = id;
            this.error = err.response.data;
            this.stream = null;
            this.running = false;
          } else throw err;
        });
    },
  },
});
