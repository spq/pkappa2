import { TypedEmitter } from "tiny-typed-emitter";
import { ConverterStatistics } from "@/apiClient";

interface GlobalEvents {
  setSearchTerm: (searchTerm: string) => void;
  showConverterResetDialog: (converter: ConverterStatistics) => void;
  showCreateTagDialog: (
    tagType: string,
    tagQuery: string,
    tagStreams: number[]
  ) => void;
  showError: (message: string) => void;
  showTagColorChangeDialog: (tagId: string) => void;
  showTagDeleteDialog: (tagId: string) => void;
  showTagDetailsDialog: (tagId: string) => void;
  showTagSetConvertersDialog: (tagId: string) => void;
  showCTFWizard: () => void;
}

export const EventBus = new TypedEmitter<GlobalEvents>();
