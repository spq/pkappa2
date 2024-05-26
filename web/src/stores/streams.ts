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
}

export const useStreamsStore = defineStore("streams", {
  state: (): State => ({
    query: null,
    page: null,
    running: false,
    error: null,
    result: null,
  }),
  actions: {
    async searchStreams(query: string, page: number) {
      if (!page) page = 0;
      this.query = query;
      this.page = page;
      this.running = true;
      this.error = null;
      this.result = null;
      return APIClient.searchStreams(query, page)
        .then((data) => {
          if ("Error" in data) {
            this.error = data.Error;
            this.result = null;
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
          if (
            axios.isAxiosError<string, unknown>(err) &&
            err.response !== undefined
          ) {
            this.query = query;
            this.page = page;
            this.running = false;
            this.error = err.response.data;
            this.result = null;
          } else throw err;
        });
    },
  },
});
