// Package model defines the core entities, status enums and domain errors of
// the collation workbench. A collation project ties a base witness to one or
// more variant witnesses, aligns them through anchors, aggregates differences
// into variant loci, records editorial decisions and freezes definitive
// snapshots.
package model

import "time"

// Status enums. Each type is stored as a string in SQLite and validated on
// write, so illegal transitions are rejected at the boundary.

type ProjectStatus string

const (
	ProjectDraft      ProjectStatus = "draft"      // 草稿：工程已建，底本未设或未导入
	ProjectAligning   ProjectStatus = "aligning"   // 对齐中：锚点与区间对齐进行中
	ProjectReviewable ProjectStatus = "reviewable" // 可复核：所有异文位已有决定
	ProjectPublished  ProjectStatus = "published"  // 已发布：定本快照已发布
	ProjectArchived   ProjectStatus = "archived"   // 已封存：只读
)

type WitnessStatus string

const (
	WitnessPendingSegment  WitnessStatus = "pending_segmentation"   // 待分段
	WitnessPendingAlign    WitnessStatus = "pending_alignment"      // 待对齐
	WitnessAligned         WitnessStatus = "aligned"                // 已对齐
	WitnessNeedsManual     WitnessStatus = "needs_manual_anchor"    // 需人工锚定
	WitnessFrozen          WitnessStatus = "frozen"                 // 已冻结
)

type AnchorStatus string

const (
	AnchorCandidate   AnchorStatus = "candidate"   // 候选
	AnchorConfirmed   AnchorStatus = "confirmed"   // 已确认
	AnchorConflict    AnchorStatus = "conflict"    // 冲突
	AnchorDeprecated  AnchorStatus = "deprecated"  // 已废弃
)

type VariantStatus string

const (
	VariantUnhandled    VariantStatus = "unhandled"     // 未处理
	VariantClaimed      VariantStatus = "claimed"       // 认领中
	VariantPending      VariantStatus = "pending_review" // 待复核
	VariantDecided      VariantStatus = "decided"       // 已定夺
	VariantSuperseded   VariantStatus = "superseded"    // 已替代
)

type DiffType string

const (
	DiffWord      DiffType = "word"      // 文字差异
	DiffOmission  DiffType = "omission"  // 缺佚
	DiffAddition  DiffType = "addition"  // 增补
	DiffMovement  DiffType = "movement"  // 移动
)

type DecisionStatus string

const (
	DecisionProposed   DecisionStatus = "proposed"   // 提议
	DecisionApproved   DecisionStatus = "approved"   // 复核通过
	DecisionRejected   DecisionStatus = "rejected"   // 退回
	DecisionWithdrawn  DecisionStatus = "withdrawn"  // 撤销
)

type SnapshotStatus string

const (
	SnapshotBuilding     SnapshotStatus = "building"      // 构建中
	SnapshotPendingP     SnapshotStatus = "pending_publish" // 待发布
	SnapshotPublished    SnapshotStatus = "published"     // 已发布
	SnapshotSuperseded   SnapshotStatus = "superseded"    // 已替代
)

// Project is a collation project tying a base witness to variant witnesses.
type Project struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Status        ProjectStatus `json:"status"`
	BaseWitnessID string        `json:"base_witness_id,omitempty"`
	Version       int64         `json:"version"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// Witness is a single textual witness (a manuscript edition of the same work).
type Witness struct {
	ID                 string       `json:"id"`
	ProjectID          string       `json:"project_id"`
	Code               string       `json:"code"`
	Title              string       `json:"title"`
	BibliographicInfo  string       `json:"bibliographic_info"`
	Status             WitnessStatus `json:"status"`
	SourceTextHash     string       `json:"source_text_hash"`
	IsBase             bool         `json:"is_base"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

// Chapter groups passages inside a witness.
type Chapter struct {
	ID        string    `json:"id"`
	WitnessID string    `json:"witness_id"`
	Ordinal   int       `json:"ordinal"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// Passage is a segment of text with a character range and hash.
type Passage struct {
	ID         string    `json:"id"`
	WitnessID  string    `json:"witness_id"`
	ChapterID  string    `json:"chapter_id"`
	Ordinal    int       `json:"ordinal"`
	StartChar  int       `json:"start_char"`
	EndChar    int       `json:"end_char"`
	Text       string    `json:"text"`
	TextHash   string    `json:"text_hash"`
	CreatedAt  time.Time `json:"created_at"`
}

// Anchor connects a base passage with one or more witness passages.
type Anchor struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	BasePassageID string       `json:"base_passage_id"`
	Status        AnchorStatus `json:"status"`
	Version       int64        `json:"version"`
	Note          string       `json:"note,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	ConfirmedAt   *time.Time   `json:"confirmed_at,omitempty"`
}

// AnchorLink records which witness passage is linked by an anchor.
type AnchorLink struct {
	AnchorID  string `json:"anchor_id"`
	WitnessID string `json:"witness_id"`
	PassageID string `json:"passage_id"`
}

// Variant is a collation locus aggregating differences between anchors.
type Variant struct {
	ID              string       `json:"id"`
	ProjectID       string       `json:"project_id"`
	AnchorBeforeID  string       `json:"anchor_before_id,omitempty"`
	AnchorAfterID   string       `json:"anchor_after_id,omitempty"`
	BasePassageID   string       `json:"base_passage_id,omitempty"`
	BaseStartChar   int          `json:"base_start_char"`
	BaseEndChar     int          `json:"base_end_char"`
	DiffType        DiffType     `json:"diff_type"`
	Status          VariantStatus `json:"status"`
	ClaimedBy       string       `json:"claimed_by,omitempty"`
	LeaseVersion    int64        `json:"lease_version"`
	LeaseExpiresAt  *time.Time   `json:"lease_expires_at,omitempty"`
	Version         int64        `json:"version"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// Reading is a candidate reading proposed for a variant.
type Reading struct {
	ID        string   `json:"id"`
	VariantID string   `json:"variant_id"`
	WitnessID string   `json:"witness_id"`
	PassageID string   `json:"passage_id,omitempty"`
	Text      string   `json:"text"`
	DiffType  DiffType `json:"diff_type"`
	CreatedAt time.Time `json:"created_at"`
}

// Decision records an editorial decision adopting a reading.
type Decision struct {
	ID                string         `json:"id"`
	VariantID         string         `json:"variant_id"`
	ReadingID         string         `json:"reading_id"`
	Status            DecisionStatus `json:"status"`
	Reason            string         `json:"reason"`
	DecidedBy         string         `json:"decided_by"`
	EvidenceWitnessID string         `json:"evidence_witness_id,omitempty"`
	Version           int64          `json:"version"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// Snapshot is a frozen definitive-text snapshot.
type Snapshot struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"project_id"`
	RoundNo     int            `json:"round_no"`
	Status      SnapshotStatus `json:"status"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	CreatedAt   time.Time      `json:"created_at"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	Version     int64          `json:"version"`
}

// SnapshotLink ties a snapshot to anchors/decisions/passage hashes so the
// frozen chain stays queryable even after later edits.
type SnapshotLink struct {
	SnapshotID string `json:"snapshot_id"`
	Kind       string `json:"kind"` // anchor | decision | passage_hash | integrity_hash
	RefID      string `json:"ref_id"`
	Payload    string `json:"payload,omitempty"`
}
