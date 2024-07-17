const STORAGE_KEY = "pkappa2_search_history";

function now() {
  return new Date().getTime();
}

/** @see {isSearches} ts-auto-guard:type-guard */
type Searches = Record<string, number>;

// TODO: Use ts-auto-guard somehow to generate this.
function isSearches(obj: unknown): obj is Searches {
  const typedObj = obj as Searches;
  return (
    typeof typedObj === "object" &&
    Object.entries(typedObj).every(
      ([key, value]) => typeof key === "string" && typeof value === "number"
    )
  );
}

function getSearches() {
  const searches: unknown = JSON.parse(
    localStorage.getItem(STORAGE_KEY) ?? "{}"
  );
  if (isSearches(searches)) {
    return searches;
  }
  throw new Error("Invalid search history");
}

function getMostRecentSearchTerms() {
  return Object.entries(getSearches())
    .sort((a, b) => b[1] - a[1])
    .map(([term]) => term);
}

function updateSearches(searches: Searches) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(searches));
}

export function addSearch(term: string) {
  const trimmedSearch = term.trim();
  if (trimmedSearch === "") {
    return;
  }
  const searches = getSearches();
  searches[trimmedSearch] = now();
  updateSearches(searches);
}

export function getTermAt(index: number) {
  const searches = getMostRecentSearchTerms();

  return searches[index];
}

export function getLastTerms(num = 10) {
  return getMostRecentSearchTerms().slice(0, num);
}

export default {
  addSearch,
  getTermAt,
  getLastTerms,
};
