package types

type SortCriteria string

const (
	Relevance     = SortCriteria("RELEVANCE")
	UpdateTime    = SortCriteria("UPDATE_TIME")
	AlphabeticAsc = SortCriteria("ALPHABETIC_ASC")
	AlphabeticDsc = SortCriteria("ALPHABETIC_DSC")
	Stars         = SortCriteria("STARS")
	Downloads     = SortCriteria("DOWNLOADS")
)

func SortFunctions() map[SortCriteria]func(pageBuffer []DetailedRepoMeta) func(i, j int) bool {
	return map[SortCriteria]func(pageBuffer []DetailedRepoMeta) func(i, j int) bool{
		AlphabeticAsc: SortByAlphabeticAsc,
		AlphabeticDsc: SortByAlphabeticDsc,
		Relevance:     SortByRelevance,
		UpdateTime:    SortByUpdateTime,
		Downloads:     SortByDownloads,
	}
}

func SortByAlphabeticAsc(pageBuffer []DetailedRepoMeta) func(i, j int) bool {
	return func(i, j int) bool {
		return pageBuffer[i].Name < pageBuffer[j].Name
	}
}

func SortByAlphabeticDsc(pageBuffer []DetailedRepoMeta) func(i, j int) bool {
	return func(i, j int) bool {
		return pageBuffer[i].Name > pageBuffer[j].Name
	}
}

func SortByRelevance(pageBuffer []DetailedRepoMeta) func(i, j int) bool {
	return func(i, j int) bool {
		return pageBuffer[i].Rank < pageBuffer[j].Rank
	}
}

// SortByUpdateTime sorting descending by time.
func SortByUpdateTime(pageBuffer []DetailedRepoMeta) func(i, j int) bool {
	return func(i, j int) bool {
		return pageBuffer[i].UpdateTime.After(pageBuffer[j].UpdateTime)
	}
}

// SortByDownloads returns a comparison function for descendant sorting by downloads.
func SortByDownloads(pageBuffer []DetailedRepoMeta) func(i, j int) bool {
	return func(i, j int) bool {
		return pageBuffer[i].Downloads > pageBuffer[j].Downloads
	}
}
