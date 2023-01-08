const STORAGE_KEY = "pkappa2_search_history";

function now() {
    return new Date().getTime();
}

function getSearches() {
    return JSON.parse(localStorage.getItem(STORAGE_KEY)) ?? {};
}

function getMostRecentSearchTerms() {
    return Object.entries(getSearches())
        .sort((a,b) => b[1] - a[1])
        .map(([term,]) => term);
}

function updateSearches(searches) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(searches));
}

export function addSearch(term) {
    const trimmedSearch = term.trim();
    if (trimmedSearch === '') {
        return;
    }
    const searches = getSearches();
    searches[trimmedSearch] = now();
    updateSearches(searches);
}

export function getTermAt(index) {
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
