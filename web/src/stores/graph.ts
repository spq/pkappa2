import { defineStore } from "pinia";
import axios from "axios";
import APIClient from "@/apiClient";
import { GraphResponse } from "@/apiClient";

export type GraphType =
  | "Active Connections"
  | "Started Connections"
  | "Finished Connections"
  | "Total Traffic"
  | "Average Traffic"
  | "Total Duration"
  | "Average Duration";

interface State {
  type: GraphType | null;
  delta: string | null;
  aspects: string[] | null;
  tags: string[] | null;
  query: string | null;
  running: boolean;
  error: string | null;
  graph: GraphResponse | null;
}

export const useGraphStore = defineStore("graph", {
  state: (): State => ({
    type: null,
    delta: null,
    aspects: null,
    tags: null,
    query: null,
    running: false,
    error: null,
    graph: null,
  }),
  actions: {
    async fetchGraph(
      delta: string,
      aspects: string[],
      tags: string[],
      query: string,
      type: GraphType
    ) {
      this.delta = delta;
      this.aspects = aspects;
      this.tags = tags;
      this.query = query;
      this.type = type;
      this.running = true;
      this.error = null;
      this.graph = null;
      return APIClient.getGraph(delta, aspects, tags, query)
        .then((data) => {
          this.delta = delta;
          this.aspects = aspects;
          this.tags = tags;
          this.query = query;
          this.type = type;
          this.error = null;
          this.graph = data;
          this.running = false;
        })
        .catch((err) => {
          if (axios.isCancel(err)) return;
          if (
            axios.isAxiosError<string, unknown>(err) &&
            err.response !== undefined
          ) {
            this.delta = delta;
            this.aspects = aspects;
            this.tags = tags;
            this.query = query;
            this.type = type;
            this.error = err.response.data;
            this.graph = null;
            this.running = false;
          } else throw err;
        });
    },
  },
});
