import { defineStore } from "pinia";
import axios from "axios";
import APIClient from "@/apiClient";
import { SearchResult } from "@/apiClient";

interface State {
  query: string | null;
  page: number | null;
  running: boolean;
  error: string | null;
  result: SearchResult | null;
  outdated: boolean;
}

export const useStreamsStore = defineStore("streams", {
  state: (): State => ({
    query: null,
    page: null,
    running: false,
    error: null,
    result: null,
    outdated: false,
  }),
  actions: {
    async searchStreams(query: string, page: number) {
      if (!page) page = 0;
      this.query = query;
      this.page = page;
      this.running = true;
      this.error = null;
      this.result = null;
      this.outdated = false;
      return APIClient.searchStreams(query, page)
        .then((data) => {
          if ("Error" in data) {
            this.error = data.Error;
            this.result = null;
            this.outdated = false;
          } else {
            this.error = null;
            this.result = data;
          }
          this.query = query;
          this.page = page;
          this.running = false;
        })
        .catch((err) => {
          if (axios.isCancel(err)) return;
          if (axios.isAxiosError<string, unknown>(err)) {
            this.query = query;
            this.page = page;
            this.running = false;
            this.error =
              err.response !== undefined && err.response.data !== ""
                ? err.response.data
                : err.message;
            this.result = null;
            this.outdated = false;
          } else throw err;
        });
    },
  },
});
