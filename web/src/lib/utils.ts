import { Data } from "@/apiClient";

export const escapeRegex =
  /[^ !#%&',/0123456789:;<=>ABCDEFGHIJKLMNOPQRSTUVWXYZ_`abcdefghijklmnopqrstuvwxyz~-]/;

export function escape(text: string) {
  return text
    .split("")
    .map((char) =>
      char.replace(
        escapeRegex,
        (match) =>
          `\\x{${match
            .charCodeAt(0)
            .toString(16)
            .toUpperCase()
            .padStart(2, "0")}}`,
      ),
    )
    .join("");
}

export const tryURLDecodeIfEnabled = (chunkData: string, enabled: boolean) => {
  if (!enabled) {
    return chunkData;
  }
  try {
    return decodeURIComponent(chunkData);
  } catch (e) {
    console.error("Failed to URL decode chunk:", e);
    return chunkData;
  }
};

export const handleUnicodeDecode = (chunk: Data, urlDecode: boolean) => {
  const chunkData = tryURLDecodeIfEnabled(atob(chunk.Content), urlDecode);
  const bytes = new Uint8Array([...chunkData].map((c) => c.charCodeAt(0)));
  return new TextDecoder("utf-8").decode(bytes);
};

export const decodeChunkContent = (
  chunk: Data,
  presentation: string,
  urlDecode: boolean,
) => {
  if (presentation === "utf-8") {
    return handleUnicodeDecode(chunk, urlDecode);
  } else if (presentation === "ascii") {
    return tryURLDecodeIfEnabled(atob(chunk.Content), urlDecode);
  }
  throw new Error(`Unsupported presentation type: ${presentation}.`);
};
