package convert_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	godigest "github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	. "github.com/smartystreets/goconvey/convey"

	"zotregistry.io/zot/pkg/extensions/search/convert"
	cvemodel "zotregistry.io/zot/pkg/extensions/search/cve/model"
	"zotregistry.io/zot/pkg/extensions/search/gql_generated"
	"zotregistry.io/zot/pkg/log"
	"zotregistry.io/zot/pkg/meta/boltdb"
	mTypes "zotregistry.io/zot/pkg/meta/types"
	"zotregistry.io/zot/pkg/test/mocks"
)

var ErrTestError = errors.New("TestError")

func TestConvertErrors(t *testing.T) {
	Convey("Convert Errors", t, func() {
		params := boltdb.DBParameters{
			RootDir: t.TempDir(),
		}
		boltDB, err := boltdb.GetBoltDriver(params)
		So(err, ShouldBeNil)

		metaDB, err := boltdb.New(boltDB, log.NewLogger("debug", ""))
		So(err, ShouldBeNil)

		configBlob, err := json.Marshal(ispec.Image{})
		So(err, ShouldBeNil)

		manifestBlob, err := json.Marshal(ispec.Manifest{
			Layers: []ispec.Descriptor{
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Size:      0,
					Digest:    godigest.NewDigestFromEncoded(godigest.SHA256, "digest"),
				},
			},
		})
		So(err, ShouldBeNil)

		repoMeta11 := mTypes.ManifestMetadata{
			ManifestBlob: manifestBlob,
			ConfigBlob:   configBlob,
		}

		digest11 := godigest.FromString("abc1")
		err = metaDB.SetManifestMeta("repo1", digest11, repoMeta11)
		So(err, ShouldBeNil)
		err = metaDB.SetRepoReference("repo1", "0.1.0", digest11, ispec.MediaTypeImageManifest)
		So(err, ShouldBeNil)

		repoMetas, manifestMetaMap, _, _, err := metaDB.SearchRepos(context.Background(), "", mTypes.Filter{},
			mTypes.PageInput{})
		So(err, ShouldBeNil)

		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		_ = convert.RepoMeta2RepoSummary(
			ctx,
			repoMetas[0],
			manifestMetaMap,
			map[string]mTypes.IndexData{},
			convert.SkipQGLField{},
			mocks.CveInfoMock{
				GetCVESummaryForImageMediaFn: func(repo string, digest, mediaType string,
				) (cvemodel.ImageCVESummary, error) {
					return cvemodel.ImageCVESummary{}, ErrTestError
				},
			},
		)

		So(graphql.GetErrors(ctx).Error(), ShouldContainSubstring, "unable to run vulnerability scan on tag")
	})

	Convey("ImageIndex2ImageSummary errors", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		_, _, err := convert.ImageIndex2ImageSummary(
			ctx,
			"repo",
			"tag",
			godigest.FromString("indexDigest"),
			true,
			mTypes.RepoMetadata{},
			mTypes.IndexData{
				IndexBlob: []byte("bad json"),
			},
			map[string]mTypes.ManifestMetadata{},
			mocks.CveInfoMock{},
		)
		So(err, ShouldNotBeNil)
	})

	Convey("ImageIndex2ImageSummary cve scanning", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		_, _, err := convert.ImageIndex2ImageSummary(
			ctx,
			"repo",
			"tag",
			godigest.FromString("indexDigest"),
			false,
			mTypes.RepoMetadata{},
			mTypes.IndexData{
				IndexBlob: []byte("{}"),
			},
			map[string]mTypes.ManifestMetadata{},
			mocks.CveInfoMock{
				GetCVESummaryForImageMediaFn: func(repo, digest, mediaType string) (cvemodel.ImageCVESummary, error) {
					return cvemodel.ImageCVESummary{}, ErrTestError
				},
			},
		)
		So(err, ShouldBeNil)
	})

	Convey("ImageManifest2ImageSummary", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)
		configBlob, err := json.Marshal(ispec.Image{
			Platform: ispec.Platform{
				OS:           "os",
				Architecture: "arch",
				Variant:      "var",
			},
		})
		So(err, ShouldBeNil)

		_, _, err = convert.ImageManifest2ImageSummary(
			ctx,
			"repo",
			"tag",
			godigest.FromString("manifestDigest"),
			false,
			mTypes.RepoMetadata{},
			mTypes.ManifestMetadata{
				ManifestBlob: []byte("{}"),
				ConfigBlob:   configBlob,
			},
			mocks.CveInfoMock{
				GetCVESummaryForImageMediaFn: func(repo, digest, mediaType string) (cvemodel.ImageCVESummary, error) {
					return cvemodel.ImageCVESummary{}, ErrTestError
				},
			},
		)
		So(err, ShouldBeNil)
	})

	Convey("ImageManifest2ManifestSummary", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		// with bad config json, error while unmarshaling
		_, _, err := convert.ImageManifest2ManifestSummary(
			ctx,
			"repo",
			"tag",
			ispec.Descriptor{
				Digest:    "dig",
				MediaType: ispec.MediaTypeImageManifest,
			},
			false,
			mTypes.RepoMetadata{
				Tags:       map[string]mTypes.Descriptor{},
				Statistics: map[string]mTypes.DescriptorStatistics{},
				Signatures: map[string]mTypes.ManifestSignatures{},
				Referrers:  map[string][]mTypes.ReferrerInfo{},
			},
			mTypes.ManifestMetadata{
				ManifestBlob: []byte(`{}`),
				ConfigBlob:   []byte("bad json"),
			},
			nil,
			mocks.CveInfoMock{},
		)
		So(err, ShouldNotBeNil)

		// CVE scan using platform
		configBlob, err := json.Marshal(ispec.Image{
			Platform: ispec.Platform{
				OS:           "os",
				Architecture: "arch",
				Variant:      "var",
			},
		})
		So(err, ShouldBeNil)

		_, _, err = convert.ImageManifest2ManifestSummary(
			ctx,
			"repo",
			"tag",
			ispec.Descriptor{
				Digest:    "dig",
				MediaType: ispec.MediaTypeImageManifest,
			},
			false,
			mTypes.RepoMetadata{
				Tags:       map[string]mTypes.Descriptor{},
				Statistics: map[string]mTypes.DescriptorStatistics{},
				Signatures: map[string]mTypes.ManifestSignatures{"dig": {"cosine": []mTypes.SignatureInfo{{}}}},
				Referrers:  map[string][]mTypes.ReferrerInfo{},
			},
			mTypes.ManifestMetadata{
				ManifestBlob: []byte("{}"),
				ConfigBlob:   configBlob,
			},
			nil,
			mocks.CveInfoMock{
				GetCVESummaryForImageMediaFn: func(repo, digest, mediaType string) (cvemodel.ImageCVESummary, error) {
					return cvemodel.ImageCVESummary{}, ErrTestError
				},
			},
		)
		So(err, ShouldBeNil)
	})

	Convey("RepoMeta2ExpandedRepoInfo", t, func() {
		ctx := graphql.WithResponseContext(context.Background(),
			graphql.DefaultErrorPresenter, graphql.DefaultRecover)

		// with bad config json, error while unmarshaling
		_, imageSummaries := convert.RepoMeta2ExpandedRepoInfo(
			ctx,
			mTypes.RepoMetadata{
				Tags: map[string]mTypes.Descriptor{
					"tag1": {Digest: "dig", MediaType: ispec.MediaTypeImageManifest},
				},
			},
			map[string]mTypes.ManifestMetadata{
				"dig": {
					ManifestBlob: []byte("{}"),
					ConfigBlob:   []byte("bad json"),
				},
			},
			map[string]mTypes.IndexData{},
			convert.SkipQGLField{
				Vulnerabilities: false,
			},
			mocks.CveInfoMock{
				GetCVESummaryForImageMediaFn: func(repo, digest, mediaType string) (cvemodel.ImageCVESummary, error) {
					return cvemodel.ImageCVESummary{}, ErrTestError
				},
			}, log.NewLogger("debug", ""),
		)
		So(len(imageSummaries), ShouldEqual, 0)

		// cveInfo present no error
		_, imageSummaries = convert.RepoMeta2ExpandedRepoInfo(
			ctx,
			mTypes.RepoMetadata{
				Tags: map[string]mTypes.Descriptor{
					"tag1": {Digest: "dig", MediaType: ispec.MediaTypeImageManifest},
				},
			},
			map[string]mTypes.ManifestMetadata{
				"dig": {
					ManifestBlob: []byte("{}"),
					ConfigBlob:   []byte("{}"),
				},
			},
			map[string]mTypes.IndexData{},
			convert.SkipQGLField{
				Vulnerabilities: false,
			},
			mocks.CveInfoMock{
				GetCVESummaryForImageMediaFn: func(repo, digest, mediaType string) (cvemodel.ImageCVESummary, error) {
					return cvemodel.ImageCVESummary{}, ErrTestError
				},
			}, log.NewLogger("debug", ""),
		)
		So(len(imageSummaries), ShouldEqual, 1)
	})
}

func TestUpdateLastUpdatedTimestam(t *testing.T) {
	Convey("Image summary is the first image checked for the repo", t, func() {
		before := time.Time{}
		after := time.Date(2023, time.April, 1, 11, 0, 0, 0, time.UTC)
		img := convert.UpdateLastUpdatedTimestamp(
			&before,
			&gql_generated.ImageSummary{LastUpdated: &before},
			&gql_generated.ImageSummary{LastUpdated: &after},
		)

		So(*img.LastUpdated, ShouldResemble, after)
	})

	Convey("Image summary is updated after the current latest image", t, func() {
		before := time.Date(2022, time.April, 1, 11, 0, 0, 0, time.UTC)
		after := time.Date(2023, time.April, 1, 11, 0, 0, 0, time.UTC)
		img := convert.UpdateLastUpdatedTimestamp(
			&before,
			&gql_generated.ImageSummary{LastUpdated: &before},
			&gql_generated.ImageSummary{LastUpdated: &after},
		)

		So(*img.LastUpdated, ShouldResemble, after)
	})

	Convey("Image summary is updated before the current latest image", t, func() {
		before := time.Date(2022, time.April, 1, 11, 0, 0, 0, time.UTC)
		after := time.Date(2023, time.April, 1, 11, 0, 0, 0, time.UTC)
		img := convert.UpdateLastUpdatedTimestamp(
			&after,
			&gql_generated.ImageSummary{LastUpdated: &after},
			&gql_generated.ImageSummary{LastUpdated: &before},
		)

		So(*img.LastUpdated, ShouldResemble, after)
	})
}

func TestLabels(t *testing.T) {
	Convey("Test labels", t, func() {
		// Test various labels
		labels := make(map[string]string)

		desc := convert.GetDescription(labels)
		So(desc, ShouldEqual, "")

		license := convert.GetLicenses(labels)
		So(license, ShouldEqual, "")

		vendor := convert.GetVendor(labels)
		So(vendor, ShouldEqual, "")

		categories := convert.GetCategories(labels)
		So(categories, ShouldEqual, "")

		labels[ispec.AnnotationVendor] = "zot"
		labels[ispec.AnnotationDescription] = "zot-desc"
		labels[ispec.AnnotationLicenses] = "zot-license"
		labels[convert.AnnotationLabels] = "zot-labels"

		desc = convert.GetDescription(labels)
		So(desc, ShouldEqual, "zot-desc")

		license = convert.GetLicenses(labels)
		So(license, ShouldEqual, "zot-license")

		vendor = convert.GetVendor(labels)
		So(vendor, ShouldEqual, "zot")

		categories = convert.GetCategories(labels)
		So(categories, ShouldEqual, "zot-labels")

		labels = make(map[string]string)

		// Use diff key
		labels[convert.LabelAnnotationVendor] = "zot-vendor"
		labels[convert.LabelAnnotationDescription] = "zot-label-desc"
		labels[ispec.AnnotationLicenses] = "zot-label-license"

		desc = convert.GetDescription(labels)
		So(desc, ShouldEqual, "zot-label-desc")

		license = convert.GetLicenses(labels)
		So(license, ShouldEqual, "zot-label-license")

		vendor = convert.GetVendor(labels)
		So(vendor, ShouldEqual, "zot-vendor")
	})
}

func TestGetSignaturesInfo(t *testing.T) {
	Convey("Test get signatures info - cosign", t, func() {
		indexDigest := godigest.FromString("123")
		repoMeta := mTypes.RepoMetadata{
			Signatures: map[string]mTypes.ManifestSignatures{string(indexDigest): {"cosign": []mTypes.SignatureInfo{{
				LayersInfo: []mTypes.LayerInfo{{LayerContent: []byte{}, LayerDigest: "", SignatureKey: "", Signer: "author"}},
			}}}},
		}

		signaturesSummary := convert.GetSignaturesInfo(true, repoMeta, indexDigest)
		So(signaturesSummary, ShouldNotBeEmpty)
		So(*signaturesSummary[0].Author, ShouldEqual, "author")
		So(*signaturesSummary[0].IsTrusted, ShouldEqual, true)
		So(*signaturesSummary[0].Tool, ShouldEqual, "cosign")
	})

	Convey("Test get signatures info - notation", t, func() {
		indexDigest := godigest.FromString("123")
		repoMeta := mTypes.RepoMetadata{
			Signatures: map[string]mTypes.ManifestSignatures{string(indexDigest): {"notation": []mTypes.SignatureInfo{{
				LayersInfo: []mTypes.LayerInfo{
					{
						LayerContent: []byte{},
						LayerDigest:  "",
						SignatureKey: "",
						Signer:       "author",
						Date:         time.Now().AddDate(0, 0, -1),
					},
				},
			}}}},
		}

		signaturesSummary := convert.GetSignaturesInfo(true, repoMeta, indexDigest)
		So(signaturesSummary, ShouldNotBeEmpty)
		So(*signaturesSummary[0].Author, ShouldEqual, "author")
		So(*signaturesSummary[0].IsTrusted, ShouldEqual, false)
		So(*signaturesSummary[0].Tool, ShouldEqual, "notation")
	})
}
