package pagination

import (
	"fmt"
	"sort"

	zerr "zotregistry.io/zot/errors"
	"zotregistry.io/zot/pkg/common"
	mTypes "zotregistry.io/zot/pkg/meta/types"
)

// PageFinder permits keeping a pool of objects using Add
// and returning a specific page.
type PageFinder interface {
	Add(detailedRepoMeta mTypes.DetailedRepoMeta)
	Page() ([]mTypes.RepoMetadata, common.PageInfo)
	Reset()
}

// RepoPageFinder implements PageFinder. It manages RepoMeta objects and calculates the page
// using the given limit, offset and sortBy option.
type RepoPageFinder struct {
	limit      int
	offset     int
	sortBy     mTypes.SortCriteria
	pageBuffer []mTypes.DetailedRepoMeta
}

func NewBaseRepoPageFinder(limit, offset int, sortBy mTypes.SortCriteria) (*RepoPageFinder, error) {
	if sortBy == "" {
		sortBy = mTypes.AlphabeticAsc
	}

	if limit < 0 {
		return nil, zerr.ErrLimitIsNegative
	}

	if offset < 0 {
		return nil, zerr.ErrOffsetIsNegative
	}

	if _, found := mTypes.SortFunctions()[sortBy]; !found {
		return nil, fmt.Errorf("sorting repos by '%s' is not supported %w",
			sortBy, zerr.ErrSortCriteriaNotSupported)
	}

	return &RepoPageFinder{
		limit:      limit,
		offset:     offset,
		sortBy:     sortBy,
		pageBuffer: make([]mTypes.DetailedRepoMeta, 0, limit),
	}, nil
}

func (bpt *RepoPageFinder) Reset() {
	bpt.pageBuffer = []mTypes.DetailedRepoMeta{}
}

func (bpt *RepoPageFinder) Add(namedRepoMeta mTypes.DetailedRepoMeta) {
	bpt.pageBuffer = append(bpt.pageBuffer, namedRepoMeta)
}

func (bpt *RepoPageFinder) Page() ([]mTypes.RepoMetadata, common.PageInfo) {
	if len(bpt.pageBuffer) == 0 {
		return []mTypes.RepoMetadata{}, common.PageInfo{}
	}

	pageInfo := &common.PageInfo{}

	sort.Slice(bpt.pageBuffer, mTypes.SortFunctions()[bpt.sortBy](bpt.pageBuffer))

	// the offset and limit are calculatd in terms of repos counted
	start := bpt.offset
	end := bpt.offset + bpt.limit

	// we'll return an empty array when the offset is greater than the number of elements
	if start >= len(bpt.pageBuffer) {
		start = len(bpt.pageBuffer)
		end = start
	}

	if end >= len(bpt.pageBuffer) {
		end = len(bpt.pageBuffer)
	}

	detailedReposPage := bpt.pageBuffer[start:end]

	pageInfo.ItemCount = len(detailedReposPage)

	if start == 0 && end == 0 {
		detailedReposPage = bpt.pageBuffer
		pageInfo.ItemCount = len(detailedReposPage)
	}

	repos := make([]mTypes.RepoMetadata, 0, len(detailedReposPage))

	for _, drm := range detailedReposPage {
		repos = append(repos, drm.RepoMetadata)
	}

	pageInfo.TotalCount = len(bpt.pageBuffer)

	return repos, *pageInfo
}

type ImagePageFinder struct {
	limit      int
	offset     int
	sortBy     mTypes.SortCriteria
	pageBuffer []mTypes.DetailedRepoMeta
}

func NewBaseImagePageFinder(limit, offset int, sortBy mTypes.SortCriteria) (*ImagePageFinder, error) {
	if sortBy == "" {
		sortBy = mTypes.AlphabeticAsc
	}

	if limit < 0 {
		return nil, zerr.ErrLimitIsNegative
	}

	if offset < 0 {
		return nil, zerr.ErrOffsetIsNegative
	}

	if _, found := mTypes.SortFunctions()[sortBy]; !found {
		return nil, fmt.Errorf("sorting repos by '%s' is not supported %w",
			sortBy, zerr.ErrSortCriteriaNotSupported)
	}

	return &ImagePageFinder{
		limit:      limit,
		offset:     offset,
		sortBy:     sortBy,
		pageBuffer: make([]mTypes.DetailedRepoMeta, 0, limit),
	}, nil
}

func (bpt *ImagePageFinder) Reset() {
	bpt.pageBuffer = []mTypes.DetailedRepoMeta{}
}

func (bpt *ImagePageFinder) Add(namedRepoMeta mTypes.DetailedRepoMeta) {
	bpt.pageBuffer = append(bpt.pageBuffer, namedRepoMeta)
}

func (bpt *ImagePageFinder) Page() ([]mTypes.RepoMetadata, common.PageInfo) {
	if len(bpt.pageBuffer) == 0 {
		return []mTypes.RepoMetadata{}, common.PageInfo{}
	}

	pageInfo := common.PageInfo{}

	for _, drm := range bpt.pageBuffer {
		repo := drm.RepoMetadata
		pageInfo.TotalCount += len(repo.Tags)
	}

	sort.Slice(bpt.pageBuffer, mTypes.SortFunctions()[bpt.sortBy](bpt.pageBuffer))

	repoStartIndex := 0
	tagStartIndex := 0

	// the offset and limit are calculatd in terms of tags counted
	remainingOffset := bpt.offset
	remainingLimit := bpt.limit

	repos := make([]mTypes.RepoMetadata, 0)

	if remainingOffset == 0 && remainingLimit == 0 {
		for _, drm := range bpt.pageBuffer {
			repo := drm.RepoMetadata
			repos = append(repos, repo)

			pageInfo.ItemCount += len(repo.Tags)
		}

		return repos, pageInfo
	}

	// bring cursor to position in RepoMeta array
	for _, drm := range bpt.pageBuffer {
		if remainingOffset < len(drm.Tags) {
			tagStartIndex = remainingOffset

			break
		}

		remainingOffset -= len(drm.Tags)
		repoStartIndex++
	}

	// offset is larger than the number of tags
	if repoStartIndex >= len(bpt.pageBuffer) {
		return []mTypes.RepoMetadata{}, common.PageInfo{}
	}

	// finish counting remaining tags inside the first repo meta
	partialTags := map[string]mTypes.Descriptor{}
	firstRepoMeta := bpt.pageBuffer[repoStartIndex].RepoMetadata

	tags := make([]string, 0, len(firstRepoMeta.Tags))
	for k := range firstRepoMeta.Tags {
		tags = append(tags, k)
	}

	sort.Strings(tags)

	for i := tagStartIndex; i < len(tags); i++ {
		tag := tags[i]

		partialTags[tag] = firstRepoMeta.Tags[tag]
		remainingLimit--

		if remainingLimit == 0 {
			firstRepoMeta.Tags = partialTags
			repos = append(repos, firstRepoMeta)
			pageInfo.ItemCount = len(partialTags)

			return repos, pageInfo
		}
	}

	firstRepoMeta.Tags = partialTags
	pageInfo.ItemCount += len(firstRepoMeta.Tags)
	repos = append(repos, firstRepoMeta)
	repoStartIndex++

	// continue with the remaining repos
	for i := repoStartIndex; i < len(bpt.pageBuffer); i++ {
		repoMeta := bpt.pageBuffer[i].RepoMetadata

		if len(repoMeta.Tags) > remainingLimit {
			partialTags := map[string]mTypes.Descriptor{}

			tags := make([]string, 0, len(repoMeta.Tags))
			for k := range repoMeta.Tags {
				tags = append(tags, k)
			}

			sort.Strings(tags)

			for _, tag := range tags {
				partialTags[tag] = repoMeta.Tags[tag]
				remainingLimit--

				if remainingLimit == 0 {
					repoMeta.Tags = partialTags
					repos = append(repos, repoMeta)

					pageInfo.ItemCount += len(partialTags)

					break
				}
			}

			return repos, pageInfo
		}

		// add the whole repo
		repos = append(repos, repoMeta)
		pageInfo.ItemCount += len(repoMeta.Tags)
		remainingLimit -= len(repoMeta.Tags)

		if remainingLimit == 0 {
			return repos, pageInfo
		}
	}

	// we arrive here when the limit is bigger than the number of tags

	return repos, pageInfo
}
