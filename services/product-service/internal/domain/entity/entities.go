package entity

import (
    "errors"
    "strings"
    "time"

    "github.com/google/uuid"
)

var (
    ErrProductNameRequired = errors.New("product name is required")
    ErrAlreadyClosed       = errors.New("engagement is already closed")
)

// ---- ProductType ----

type ProductType struct {
    ID              uuid.UUID
    Name            string // "Web Application", "Mobile App", "Infrastructure", "API"
    Description     string
    CriticalProduct bool // high-priority product types
    KeyProduct      bool // business-critical
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// ---- Product ----

type BusinessCriticality string

const (
    BCVeryHigh BusinessCriticality = "very high"
    BCHigh     BusinessCriticality = "high"
    BCMedium   BusinessCriticality = "medium"
    BCLow      BusinessCriticality = "low"
    BCVeryLow  BusinessCriticality = "very low"
)

type Lifecycle string

const (
    LCConstruction Lifecycle = "construction"
    LCProduction   Lifecycle = "production"
    LCRetirement   Lifecycle = "retirement"
)

type Platform string

const (
    PlatformWeb     Platform = "web"
    PlatformAPI     Platform = "api"
    PlatformMobile  Platform = "mobile"
    PlatformDesktop Platform = "desktop"
)

type Product struct {
    ID                         uuid.UUID
    ProductTypeID              uuid.UUID
    Name                       string
    Description                string
    ProdNumericGrade           int               // 1-100
    BusinessCriticality        BusinessCriticality
    Platform                   Platform
    Lifecycle                  Lifecycle
    Origin                     string            // internal|external|partner
    ExternalAudience           bool
    InternetAccessible         bool
    EnableFullRiskAcceptance   bool
    EnableSimpleRiskAcceptance bool
    Tags                       []string
    CreatedAt                  time.Time
    UpdatedAt                  time.Time
}

func NewProduct(productTypeID uuid.UUID, name, description string) (*Product, error) {
    if strings.TrimSpace(name) == "" {
        return nil, ErrProductNameRequired
    }
    return &Product{
        ID:                  uuid.New(),
        ProductTypeID:       productTypeID,
        Name:                strings.TrimSpace(name),
        Description:         description,
        BusinessCriticality: BCMedium,
        Platform:            PlatformWeb,
        Lifecycle:           LCProduction,
        Tags:                []string{},
        CreatedAt:           time.Now().UTC(),
        UpdatedAt:           time.Now().UTC(),
    }, nil
}

// ---- Engagement ----

type EngagementType string

const (
    TypeInteractive EngagementType = "Interactive"
    TypeCICD        EngagementType = "CI/CD"
)

type EngagementStatus string

const (
    StatusNotStarted EngagementStatus = "Not Started"
    StatusInProgress EngagementStatus = "In Progress"
    StatusOnHold     EngagementStatus = "On Hold"
    StatusCompleted  EngagementStatus = "Completed"
    StatusCancelled  EngagementStatus = "Cancelled"
)

type Engagement struct {
    ID                        uuid.UUID
    ProductID                 uuid.UUID
    Name                      string
    Description               string
    LeadID                    *uuid.UUID
    EngagementType            EngagementType
    Status                    EngagementStatus
    StartDate                 time.Time
    EndDate                   *time.Time
    Version                   string
    BuildID                   string
    CommitHash                string
    BranchTag                 string
    SourceCodeManagementURI   string
    DeduplicationOnEngagement bool
    Tags                      []string
    CreatedAt                 time.Time
    UpdatedAt                 time.Time
}

func NewEngagement(productID uuid.UUID, name string, engType EngagementType) *Engagement {
    return &Engagement{
        ID:             uuid.New(),
        ProductID:      productID,
        Name:           strings.TrimSpace(name),
        EngagementType: engType,
        Status:         StatusNotStarted,
        StartDate:      time.Now().UTC(),
        Tags:           []string{},
        CreatedAt:      time.Now().UTC(),
        UpdatedAt:      time.Now().UTC(),
    }
}

func (e *Engagement) Close() error {
    if e.Status == StatusCompleted {
        return ErrAlreadyClosed
    }
    e.Status = StatusCompleted
    now := time.Now().UTC()
    e.EndDate = &now
    e.UpdatedAt = now
    return nil
}

func (e *Engagement) IsCICD() bool { return e.EngagementType == TypeCICD }

// ---- Test ----

type Test struct {
    ID           uuid.UUID
    EngagementID uuid.UUID
    Title        string
    Description  string
    TestType     string // "nmap"|"zap"|"agent"|"manual"|"dast"|"sast"
    TargetStart  *time.Time
    TargetEnd    *time.Time
    ScanID       *uuid.UUID // Link to scan-service scan
    FindingCount int
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

func NewTest(engagementID uuid.UUID, title, testType string, scanID *uuid.UUID) *Test {
    now := time.Now().UTC()
    return &Test{
        ID:           uuid.New(),
        EngagementID: engagementID,
        Title:        strings.TrimSpace(title),
        TestType:     testType,
        ScanID:       scanID,
        CreatedAt:    now,
        UpdatedAt:    now,
    }
}
