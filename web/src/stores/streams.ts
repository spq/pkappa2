import { defineStore } from "pinia";
import axios from "axios";
import APIClient from "@/apiClient";
import { SearchResult } from "@/apiClient";

interface State {
  query: string | null;
  page: number | null;
  /** the latest page that is loaded. updated by pagination and infinite scroll */
  latestPage: number | null;
  running: boolean;
  error: string | null;
  result: SearchResult | null;
  outdated: boolean;
}

export const useStreamsStore = defineStore("streams", {
  state: (): State => ({
    query: null,
    page: null,
    latestPage: null,
    running: false,
    error: null,
    result: null,
    outdated: false,
  }),
  actions: {
    async searchStreams(query: string, page: number, append = false) {
      if (!page) page = 0;
      this.query = query;
      if (!append) this.page = page;
      this.running = true;
      this.error = null;
      if (!append) this.result = null;
      this.outdated = false;
      return APIClient.searchStreams(query, page)
        .then((data) => {
          if ("Error" in data) {
            this.error = data.Error;
            this.result = null;
            this.outdated = false;
          } else {
            this.error = null;
            if (append && this.result) {
              this.result = {
                ...data,
                Offset: this.result.Offset,
                Results: [...this.result.Results, ...data.Results],
              };
              this.latestPage = page;
            } else {
              this.result = data;
              this.latestPage = page;
            }
          }
          this.running = false;
        })
        .catch((err: unknown) => {
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
