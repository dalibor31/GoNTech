# Project.md - POS + Servis Sistem za Računarsku Radnju (Go)

## Opis projekta

Sistem za vođenje servisa računara i maloprodaje delova, alata, kablova, komponenti i gotovih računara/laptopova. Podržava:
- Magacinsko poslovanje
- Maloprodaju (sa fiskalnom kasom)
- Servisne naloge
- Nabavke od dobavljača
- Upravljanje klijentima (fizička i pravna lica)
- Izveštaje za poresku upravu

---

## Arhitektura baze podataka (PostgreSQL)

```sql
-- =============================================
-- 1. Tvoja firma
-- =============================================
CREATE TABLE companies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    address         TEXT NOT NULL,
    pib             VARCHAR(9) UNIQUE NOT NULL,
    maticni_broj    VARCHAR(8) UNIQUE NOT NULL,
    pdv_number      VARCHAR(15),
    activity_code   VARCHAR(5) NOT NULL,
    bank_account    VARCHAR(18),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 2. Korisnici sistema
-- =============================================
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      UUID NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    username        VARCHAR(100) UNIQUE NOT NULL,
    password_hash   VARCHAR(255) NOT NULL,
    role            VARCHAR(50) NOT NULL CHECK (role IN ('admin', 'manager', 'cashier', 'technician')),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 3. Dobavljači
-- =============================================
CREATE TABLE suppliers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    address         TEXT,
    pib             VARCHAR(9) UNIQUE,
    is_vat_payer    BOOLEAN DEFAULT TRUE,
    contact_person  VARCHAR(100),
    phone           VARCHAR(20),
    email           VARCHAR(255),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 4. Klijenti (fizička i pravna lica)
-- =============================================
CREATE TABLE customers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type            VARCHAR(20) NOT NULL CHECK (type IN ('individual', 'company')),
    first_name      VARCHAR(100),
    last_name       VARCHAR(100),
    jmbg            VARCHAR(13) UNIQUE,
    company_name    VARCHAR(255),
    pib             VARCHAR(9) UNIQUE,
    address         TEXT,
    phone           VARCHAR(20),
    email           VARCHAR(255),
    loyalty_points  INT DEFAULT 0,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 5. Proizvodi (delovi, alati, kablovi, komponente, računari)
-- =============================================
CREATE TABLE products (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sku                 VARCHAR(100) UNIQUE NOT NULL,
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    category            VARCHAR(100) NOT NULL,
    manufacturer        VARCHAR(100),
    cost_price_no_vat   DECIMAL(15,2) NOT NULL,
    selling_price_with_vat DECIMAL(15,2) NOT NULL,
    vat_rate            DECIMAL(5,2) DEFAULT 20.00,
    stock_quantity      INT NOT NULL DEFAULT 0,
    min_stock_level     INT DEFAULT 5,
    location            VARCHAR(50),
    warranty_months     INT DEFAULT 12,
    is_active           BOOLEAN DEFAULT TRUE,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 6. Servisni nalozi
-- =============================================
CREATE TABLE service_orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number        VARCHAR(50) UNIQUE NOT NULL,
    customer_id         UUID NOT NULL REFERENCES customers(id),
    device_type         VARCHAR(50) NOT NULL,
    device_brand        VARCHAR(100),
    device_model        VARCHAR(100),
    serial_number       VARCHAR(100),
    reported_issue      TEXT NOT NULL,
    diagnosis           TEXT,
    status              VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'diagnosis', 'waiting_parts', 'repairing', 'testing', 'ready', 'completed', 'rejected')),
    repair_cost_no_vat  DECIMAL(15,2) DEFAULT 0,
    repair_cost_with_vat DECIMAL(15,2) DEFAULT 0,
    estimated_completion DATE,
    completed_at        TIMESTAMP,
    received_by         UUID REFERENCES users(id),
    technician_id       UUID REFERENCES users(id),
    customer_notes      TEXT,
    internal_notes      TEXT,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 7. Ugrađeni delovi u servisu
-- =============================================
CREATE TABLE service_parts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_order_id    UUID NOT NULL REFERENCES service_orders(id) ON DELETE CASCADE,
    product_id          UUID NOT NULL REFERENCES products(id),
    quantity            INT NOT NULL CHECK (quantity > 0),
    unit_price_no_vat   DECIMAL(15,2) NOT NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 8. Prodajne transakcije (fiskalni računi)
-- =============================================
CREATE TABLE sales (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_number         VARCHAR(50) UNIQUE NOT NULL,
    customer_id         UUID REFERENCES customers(id),
    seller_id           UUID NOT NULL REFERENCES users(id),
    sale_type           VARCHAR(20) NOT NULL CHECK (sale_type IN ('retail_individual', 'retail_company', 'service')),
    fiscal_receipt_number VARCHAR(50),
    fiscal_qr_code      TEXT,
    payment_method      VARCHAR(20) NOT NULL CHECK (payment_method IN ('cash', 'card', 'wire_transfer')),
    total_amount_no_vat DECIMAL(15,2) NOT NULL,
    total_vat           DECIMAL(15,2) NOT NULL,
    total_amount_with_vat DECIMAL(15,2) NOT NULL,
    sale_date           TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    voided              BOOLEAN DEFAULT FALSE,
    void_reason         TEXT
);

-- =============================================
-- 9. Stavke prodaje
-- =============================================
CREATE TABLE sale_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id             UUID NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
    product_id          UUID NOT NULL REFERENCES products(id),
    quantity            INT NOT NULL CHECK (quantity > 0),
    unit_price_no_vat   DECIMAL(15,2) NOT NULL,
    unit_price_with_vat DECIMAL(15,2) NOT NULL,
    vat_amount          DECIMAL(15,2) NOT NULL,
    total_no_vat        DECIMAL(15,2) NOT NULL,
    total_with_vat      DECIMAL(15,2) NOT NULL
);

-- =============================================
-- 10. Nabavke od dobavljača
-- =============================================
CREATE TABLE purchases (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_number     VARCHAR(50) UNIQUE NOT NULL,
    supplier_id         UUID NOT NULL REFERENCES suppliers(id),
    invoice_number      VARCHAR(100),
    invoice_date        DATE,
    purchase_type       VARCHAR(20) NOT NULL CHECK (purchase_type IN ('domestic', 'import')),
    customs_declaration VARCHAR(100),
    total_amount_no_vat DECIMAL(15,2) NOT NULL,
    total_vat           DECIMAL(15,2) NOT NULL,
    total_amount_with_vat DECIMAL(15,2) NOT NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- =============================================
-- 11. Stavke nabavke
-- =============================================
CREATE TABLE purchase_items (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_id         UUID NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
    product_id          UUID NOT NULL REFERENCES products(id),
    quantity            INT NOT NULL CHECK (quantity > 0),
    unit_price_no_vat   DECIMAL(15,2) NOT NULL,
    vat_rate            DECIMAL(5,2) NOT NULL,
    total_no_vat        DECIMAL(15,2) NOT NULL,
    total_vat           DECIMAL(15,2) NOT NULL
);

-- =============================================
-- 12. Magacinske promene (revizija)
-- =============================================
CREATE TABLE warehouse_transactions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id          UUID NOT NULL REFERENCES products(id),
    transaction_type    VARCHAR(20) NOT NULL CHECK (transaction_type IN ('purchase_in', 'sale_out', 'service_out', 'return_in', 'return_out', 'adjustment')),
    reference_id        UUID NOT NULL,
    quantity_change     INT NOT NULL,
    stock_before        INT NOT NULL,
    stock_after         INT NOT NULL,
    unit_cost_no_vat    DECIMAL(15,2) NOT NULL,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by          UUID REFERENCES users(id)
);

```ž

Go tipovi podataka

// =============================================
// models/product.go
// =============================================
package models

import (
    "time"
    "github.com/google/uuid"
)

type ProductCategory string

const (
    CategoryCPU      ProductCategory = "CPU"
    CategoryRAM      ProductCategory = "RAM"
    CategoryGPU      ProductCategory = "GPU"
    CategoryStorage  ProductCategory = "STORAGE"
    CategoryCable    ProductCategory = "CABLE"
    CategoryTool     ProductCategory = "TOOL"
    CategoryLaptop   ProductCategory = "LAPTOP"
    CategoryDesktop  ProductCategory = "DESKTOP"
)

type Product struct {
    ID                  uuid.UUID       `db:"id" json:"id"`
    SKU                 string          `db:"sku" json:"sku"`
    Name                string          `db:"name" json:"name"`
    Description         string          `db:"description" json:"description"`
    Category            ProductCategory `db:"category" json:"category"`
    Manufacturer        string          `db:"manufacturer" json:"manufacturer"`
    CostPriceNoVat      float64         `db:"cost_price_no_vat" json:"cost_price_no_vat"`
    SellingPriceWithVat float64         `db:"selling_price_with_vat" json:"selling_price_with_vat"`
    VatRate             float64         `db:"vat_rate" json:"vat_rate"`
    StockQuantity       int             `db:"stock_quantity" json:"stock_quantity"`
    MinStockLevel       int             `db:"min_stock_level" json:"min_stock_level"`
    Location            string          `db:"location" json:"location"`
    WarrantyMonths      int             `db:"warranty_months" json:"warranty_months"`
    IsActive            bool            `db:"is_active" json:"is_active"`
    CreatedAt           time.Time       `db:"created_at" json:"created_at"`
    UpdatedAt           time.Time       `db:"updated_at" json:"updated_at"`
}

func (p *Product) SellingPriceNoVat() float64 {
    return p.SellingPriceWithVat / (1 + p.VatRate/100)
}

// =============================================
// models/customer.go
// =============================================
package models

import (
    "time"
    "github.com/google/uuid"
)

type CustomerType string

const (
    CustomerIndividual CustomerType = "individual"
    CustomerCompany    CustomerType = "company"
)

type Customer struct {
    ID           uuid.UUID    `db:"id" json:"id"`
    Type         CustomerType `db:"type" json:"type"`
    FirstName    string       `db:"first_name" json:"first_name,omitempty"`
    LastName     string       `db:"last_name" json:"last_name,omitempty"`
    JMBG         string       `db:"jmbg" json:"jmbg,omitempty"`
    CompanyName  string       `db:"company_name" json:"company_name,omitempty"`
    PIB          string       `db:"pib" json:"pib,omitempty"`
    Address      string       `db:"address" json:"address"`
    Phone        string       `db:"phone" json:"phone"`
    Email        string       `db:"email" json:"email"`
    LoyaltyPoints int         `db:"loyalty_points" json:"loyalty_points"`
    CreatedAt    time.Time    `db:"created_at" json:"created_at"`
}

func (c *Customer) FullName() string {
    if c.Type == CustomerIndividual {
        return c.FirstName + " " + c.LastName
    }
    return c.CompanyName
}

// =============================================
// models/service_order.go
// =============================================
package models

import (
    "time"
    "github.com/google/uuid"
)

type ServiceStatus string

const (
    StatusPending      ServiceStatus = "pending"
    StatusDiagnosis    ServiceStatus = "diagnosis"
    StatusWaitingParts ServiceStatus = "waiting_parts"
    StatusRepairing    ServiceStatus = "repairing"
    StatusTesting      ServiceStatus = "testing"
    StatusReady        ServiceStatus = "ready"
    StatusCompleted    ServiceStatus = "completed"
    StatusRejected     ServiceStatus = "rejected"
)

type ServiceOrder struct {
    ID                    uuid.UUID      `db:"id" json:"id"`
    OrderNumber           string         `db:"order_number" json:"order_number"`
    CustomerID            uuid.UUID      `db:"customer_id" json:"customer_id"`
    Customer              *Customer      `db:"-" json:"customer,omitempty"`
    DeviceType            string         `db:"device_type" json:"device_type"`
    DeviceBrand           string         `db:"device_brand" json:"device_brand"`
    DeviceModel           string         `db:"device_model" json:"device_model"`
    SerialNumber          string         `db:"serial_number" json:"serial_number"`
    ReportedIssue         string         `db:"reported_issue" json:"reported_issue"`
    Diagnosis             string         `db:"diagnosis" json:"diagnosis"`
    Status                ServiceStatus  `db:"status" json:"status"`
    RepairCostNoVat       float64        `db:"repair_cost_no_vat" json:"repair_cost_no_vat"`
    RepairCostWithVat     float64        `db:"repair_cost_with_vat" json:"repair_cost_with_vat"`
    EstimatedCompletion   *time.Time     `db:"estimated_completion" json:"estimated_completion"`
    CompletedAt           *time.Time     `db:"completed_at" json:"completed_at"`
    ReceivedBy            *uuid.UUID     `db:"received_by" json:"received_by"`
    TechnicianID          *uuid.UUID     `db:"technician_id" json:"technician_id"`
    CustomerNotes         string         `db:"customer_notes" json:"customer_notes"`
    InternalNotes         string         `db:"internal_notes" json:"internal_notes"`
    UsedParts             []ServicePart  `db:"-" json:"used_parts,omitempty"`
    CreatedAt             time.Time      `db:"created_at" json:"created_at"`
    UpdatedAt             time.Time      `db:"updated_at" json:"updated_at"`
}

type ServicePart struct {
    ID               uuid.UUID `db:"id" json:"id"`
    ServiceOrderID   uuid.UUID `db:"service_order_id" json:"service_order_id"`
    ProductID        uuid.UUID `db:"product_id" json:"product_id"`
    Product          *Product  `db:"-" json:"product,omitempty"`
    Quantity         int       `db:"quantity" json:"quantity"`
    UnitPriceNoVat   float64   `db:"unit_price_no_vat" json:"unit_price_no_vat"`
    CreatedAt        time.Time `db:"created_at" json:"created_at"`
}

// =============================================
// models/sale.go
// =============================================
package models

import (
    "time"
    "github.com/google/uuid"
)

type SaleType string
type PaymentMethod string

const (
    SaleTypeRetailIndividual SaleType = "retail_individual"
    SaleTypeRetailCompany    SaleType = "retail_company"
    SaleTypeService          SaleType = "service"
)

const (
    PaymentCash         PaymentMethod = "cash"
    PaymentCard         PaymentMethod = "card"
    PaymentWireTransfer PaymentMethod = "wire_transfer"
)

type Sale struct {
    ID                     uuid.UUID     `db:"id" json:"id"`
    SaleNumber             string        `db:"sale_number" json:"sale_number"`
    CustomerID             *uuid.UUID    `db:"customer_id" json:"customer_id"`
    Customer               *Customer     `db:"-" json:"customer,omitempty"`
    SellerID               uuid.UUID     `db:"seller_id" json:"seller_id"`
    Seller                 *User         `db:"-" json:"seller,omitempty"`
    SaleType               SaleType      `db:"sale_type" json:"sale_type"`
    FiscalReceiptNumber    string        `db:"fiscal_receipt_number" json:"fiscal_receipt_number"`
    FiscalQRCode           string        `db:"fiscal_qr_code" json:"fiscal_qr_code"`
    PaymentMethod          PaymentMethod `db:"payment_method" json:"payment_method"`
    TotalAmountNoVat       float64       `db:"total_amount_no_vat" json:"total_amount_no_vat"`
    TotalVat               float64       `db:"total_vat" json:"total_vat"`
    TotalAmountWithVat     float64       `db:"total_amount_with_vat" json:"total_amount_with_vat"`
    SaleDate               time.Time     `db:"sale_date" json:"sale_date"`
    Voided                 bool          `db:"voided" json:"voided"`
    VoidReason             string        `db:"void_reason" json:"void_reason"`
    Items                  []SaleItem    `db:"-" json:"items,omitempty"`
}

type SaleItem struct {
    ID                 uuid.UUID `db:"id" json:"id"`
    SaleID             uuid.UUID `db:"sale_id" json:"sale_id"`
    ProductID          uuid.UUID `db:"product_id" json:"product_id"`
    Product            *Product  `db:"-" json:"product,omitempty"`
    Quantity           int       `db:"quantity" json:"quantity"`
    UnitPriceNoVat     float64   `db:"unit_price_no_vat" json:"unit_price_no_vat"`
    UnitPriceWithVat   float64   `db:"unit_price_with_vat" json:"unit_price_with_vat"`
    VatAmount          float64   `db:"vat_amount" json:"vat_amount"`
    TotalNoVat         float64   `db:"total_no_vat" json:"total_no_vat"`
    TotalWithVat       float64   `db:"total_with_vat" json:"total_with_vat"`
}

```

Go tipovi podataka za nabavke i korisnike sistema su definisani u nastavku.

```go
// =============================================
// models/purchase.go
// =============================================
package models

import (
    "time"
    "github.com/google/uuid"
)

type PurchaseType string

const (
    PurchaseDomestic PurchaseType = "domestic"
    PurchaseImport   PurchaseType = "import"
)

type Purchase struct {
    ID                  uuid.UUID      `db:"id" json:"id"`
    PurchaseNumber      string         `db:"purchase_number" json:"purchase_number"`
    SupplierID          uuid.UUID      `db:"supplier_id" json:"supplier_id"`
    Supplier            *Supplier      `db:"-" json:"supplier,omitempty"`
    InvoiceNumber       string         `db:"invoice_number" json:"invoice_number"`
    InvoiceDate         *time.Time     `db:"invoice_date" json:"invoice_date"`
    PurchaseType        PurchaseType   `db:"purchase_type" json:"purchase_type"`
    CustomsDeclaration  string         `db:"customs_declaration" json:"customs_declaration"`
    TotalAmountNoVat    float64        `db:"total_amount_no_vat" json:"total_amount_no_vat"`
    TotalVat            float64        `db:"total_vat" json:"total_vat"`
    TotalAmountWithVat  float64        `db:"total_amount_with_vat" json:"total_amount_with_vat"`
    Items               []PurchaseItem `db:"-" json:"items,omitempty"`
    CreatedAt           time.Time      `db:"created_at" json:"created_at"`
}

type PurchaseItem struct {
    ID               uuid.UUID `db:"id" json:"id"`
    PurchaseID       uuid.UUID `db:"purchase_id" json:"purchase_id"`
    ProductID        uuid.UUID `db:"product_id" json:"product_id"`
    Product          *Product  `db:"-" json:"product,omitempty"`
    Quantity         int       `db:"quantity" json:"quantity"`
    UnitPriceNoVat   float64   `db:"unit_price_no_vat" json:"unit_price_no_vat"`
    VatRate          float64   `db:"vat_rate" json:"vat_rate"`
    TotalNoVat       float64   `db:"total_no_vat" json:"total_no_vat"`
    TotalVat         float64   `db:"total_vat" json:"total_vat"`
}

// =============================================
// models/user.go
// =============================================
package models

import (
    "time"
    "github.com/google/uuid"
)

type UserRole string

const (
    RoleAdmin     UserRole = "admin"
    RoleManager   UserRole = "manager"
    RoleCashier   UserRole = "cashier"
    RoleTechnician UserRole = "technician"
)

type User struct {
    ID           uuid.UUID `db:"id" json:"id"`
    CompanyID    uuid.UUID `db:"company_id" json:"company_id"`
    Username     string    `db:"username" json:"username"`
    PasswordHash string    `db:"password_hash" json:"-"`
    Role         UserRole  `db:"role" json:"role"`
    CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

```

Servisi za fiskalnu kasu, magacinsko poslovanje, izveštaje i servisne naloge su implementirani u nastavku.

```go
// =============================================
// services/fiscal_api.go - integracija sa fiskalnom kasom
// =============================================
package services

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type FiscalReceiptRequest struct {
    CashierName   string              `json:"cashier_name"`
    Items         []FiscalReceiptItem `json:"items"`
    PaymentMethod string              `json:"payment_method"`
    TotalAmount   float64             `json:"total_amount"`
}

type FiscalReceiptItem struct {
    Name        string  `json:"name"`
    Quantity    int     `json:"quantity"`
    UnitPrice   float64 `json:"unit_price"`
    VatRate     float64 `json:"vat_rate"`
    TotalAmount float64 `json:"total_amount"`
}

type FiscalReceiptResponse struct {
    ReceiptNumber string `json:"receipt_number"`
    QRCode        string `json:"qr_code"`
    Error         string `json:"error,omitempty"`
}

type FiscalAPIClient struct {
    BaseURL    string
    HTTPClient *http.Client
}

func NewFiscalAPIClient(baseURL string) *FiscalAPIClient {
    return &FiscalAPIClient{
        BaseURL:    baseURL,
        HTTPClient: &http.Client{},
    }
}

func (c *FiscalAPIClient) SendReceipt(req FiscalReceiptRequest) (*FiscalReceiptResponse, error) {
    jsonData, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    resp, err := c.HTTPClient.Post(c.BaseURL+"/api/receipt", "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to send to fiscal cash register: %w", err)
    }
    defer resp.Body.Close()

    var fiscalResp FiscalReceiptResponse
    if err := json.NewDecoder(resp.Body).Decode(&fiscalResp); err != nil {
        return nil, fmt.Errorf("failed to decode fiscal response: %w", err)
    }

    if fiscalResp.Error != "" {
        return nil, fmt.Errorf("fiscal cash register error: %s", fiscalResp.Error)
    }

    return &fiscalResp, nil
}

// =============================================
// services/warehouse.go - magacinska logika
// =============================================
package services

import (
    "database/sql"
    "fmt"
    "github.com/google/uuid"
)

type WarehouseService struct {
    DB *sql.DB
}

type WarehouseTransaction struct {
    ProductID       uuid.UUID
    TransactionType string
    ReferenceID     uuid.UUID
    QuantityChange  int
    UnitCostNoVat   float64
    UserID          uuid.UUID
}

func (s *WarehouseService) UpdateStock(tx *sql.Tx, wt WarehouseTransaction) error {
    var currentStock int
    var currentCost float64
    query := `SELECT stock_quantity, cost_price_no_vat FROM products WHERE id = $1 FOR UPDATE`
    err := tx.QueryRow(query, wt.ProductID).Scan(&currentStock, &currentCost)
    if err != nil {
        return fmt.Errorf("failed to get current stock: %w", err)
    }

    newStock := currentStock + wt.QuantityChange
    if newStock < 0 {
        return fmt.Errorf("insufficient stock: have %d, trying to remove %d", currentStock, -wt.QuantityChange)
    }

    _, err = tx.Exec(`UPDATE products SET stock_quantity = $1, cost_price_no_vat = $2, updated_at = NOW() WHERE id = $3`,
        newStock, wt.UnitCostNoVat, wt.ProductID)
    if err != nil {
        return fmt.Errorf("failed to update product stock: %w", err)
    }

    _, err = tx.Exec(`INSERT INTO warehouse_transactions 
        (product_id, transaction_type, reference_id, quantity_change, stock_before, stock_after, unit_cost_no_vat, created_by)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
        wt.ProductID, wt.TransactionType, wt.ReferenceID, wt.QuantityChange, currentStock, newStock, wt.UnitCostNoVat, wt.UserID)
    if err != nil {
        return fmt.Errorf("failed to log warehouse transaction: %w", err)
    }

    return nil
}

func (s *WarehouseService) GetLowStockProducts(threshold int) ([]Product, error) {
    query := `SELECT * FROM products WHERE stock_quantity <= $1 AND is_active = true`
    rows, err := s.DB.Query(query, threshold)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var products []Product
    for rows.Next() {
        var p Product
        err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.Category, &p.Manufacturer,
            &p.CostPriceNoVat, &p.SellingPriceWithVat, &p.VatRate, &p.StockQuantity,
            &p.MinStockLevel, &p.Location, &p.WarrantyMonths, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
        if err != nil {
            return nil, err
        }
        products = append(products, p)
    }
    return products, nil
}

// =============================================
// services/reports.go - izveštaji za poresku
// =============================================
package services

import (
    "database/sql"
    "time"
)

type ReportService struct {
    DB *sql.DB
}

type DailySalesReport struct {
    Date                   time.Time
    TotalSalesCount        int
    TotalCashAmount        float64
    TotalCardAmount        float64
    TotalWireAmount        float64
    TotalVAT20             float64
    TotalVAT10             float64
    TotalRevenueWithVAT    float64
    VoidedReceiptsCount    int
}

func (s *ReportService) GetDailySalesReport(date time.Time) (*DailySalesReport, error) {
    query := `
        SELECT 
            COUNT(*) as total_sales,
            COALESCE(SUM(CASE WHEN payment_method = 'cash' THEN total_amount_with_vat ELSE 0 END), 0) as cash_total,
            COALESCE(SUM(CASE WHEN payment_method = 'card' THEN total_amount_with_vat ELSE 0 END), 0) as card_total,
            COALESCE(SUM(CASE WHEN payment_method = 'wire_transfer' THEN total_amount_with_vat ELSE 0 END), 0) as wire_total,
            COALESCE(SUM(total_vat), 0) as total_vat,
            COALESCE(SUM(CASE WHEN voided = true THEN 1 ELSE 0 END), 0) as voided_count
        FROM sales 
        WHERE DATE(sale_date) = $1
    `

    var report DailySalesReport
    report.Date = date

    var totalVat float64
    err := s.DB.QueryRow(query, date.Format("2006-01-02")).Scan(
        &report.TotalSalesCount, &report.TotalCashAmount, &report.TotalCardAmount,
        &report.TotalWireAmount, &totalVat, &report.VoidedReceiptsCount)
    if err != nil {
        return nil, err
    }

    report.TotalRevenueWithVAT = report.TotalCashAmount + report.TotalCardAmount + report.TotalWireAmount
    report.TotalVAT20 = totalVat
    report.TotalVAT10 = 0

    return &report, nil
}

type VATReport struct {
    PeriodStart    time.Time
    PeriodEnd      time.Time
    TotalSalesNoVat  float64
    TotalSalesVat    float64
    TotalPurchasesNoVat float64
    TotalPurchasesVat   float64
    VATPayable       float64
}

func (s *ReportService) GetVATReport(startDate, endDate time.Time) (*VATReport, error) {
    report := &VATReport{
        PeriodStart: startDate,
        PeriodEnd:   endDate,
    }

    // Prodaja
    err := s.DB.QueryRow(`
        SELECT COALESCE(SUM(total_amount_no_vat), 0), COALESCE(SUM(total_vat), 0)
        FROM sales WHERE DATE(sale_date) BETWEEN $1 AND $2 AND voided = false`,
        startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).Scan(&report.TotalSalesNoVat, &report.TotalSalesVat)
    if err != nil {
        return nil, err
    }

    // Nabavke
    err = s.DB.QueryRow(`
        SELECT COALESCE(SUM(total_amount_no_vat), 0), COALESCE(SUM(total_vat), 0)
        FROM purchases WHERE DATE(created_at) BETWEEN $1 AND $2`,
        startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).Scan(&report.TotalPurchasesNoVat, &report.TotalPurchasesVat)
    if err != nil {
        return nil, err
    }

    report.VATPayable = report.TotalSalesVat - report.TotalPurchasesVat
    if report.VATPayable < 0 {
        report.VATPayable = 0
    }

    return report, nil
}

// =============================================
// services/service_order.go - servisna logika
// =============================================
package services

import (
    "database/sql"
    "fmt"
    "github.com/google/uuid"
)

type ServiceOrderService struct {
    DB               *sql.DB
    WarehouseService *WarehouseService
}

func (s *ServiceOrderService) CompleteService(orderID uuid.UUID, technicianID uuid.UUID) error {
    tx, err := s.DB.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Dohvati servisni nalog sa delovima
    var order ServiceOrder
    err = tx.QueryRow(`
        SELECT id, order_number, repair_cost_no_vat, repair_cost_with_vat, status
        FROM service_orders WHERE id = $1 FOR UPDATE`, orderID).Scan(
        &order.ID, &order.OrderNumber, &order.RepairCostNoVat, &order.RepairCostWithVat, &order.Status)
    if err != nil {
        return err
    }

    if order.Status != StatusReady {
        return fmt.Errorf("service order is not ready for completion, current status: %s", order.Status)
    }

    // Skini delove iz magacina
    rows, err := tx.Query(`SELECT product_id, quantity, unit_price_no_vat FROM service_parts WHERE service_order_id = $1`, orderID)
    if err != nil {
        return err
    }
    defer rows.Close()

    for rows.Next() {
        var productID uuid.UUID
        var quantity int
        var unitPrice float64
        if err := rows.Scan(&productID, &quantity, &unitPrice); err != nil {
            return err
        }

        wt := WarehouseTransaction{
            ProductID:      productID,
            TransactionType: "service_out",
            ReferenceID:    orderID,
            QuantityChange: -quantity,
            UnitCostNoVat:  unitPrice,
            UserID:         technicianID,
        }
        if err := s.WarehouseService.UpdateStock(tx, wt); err != nil {
            return err
        }
    }

    // Ako servis ima cenu rada > 0, kreiraj prodaju (usluga)
    if order.RepairCostWithVat > 0 {
        // Ovo bi trebalo da ide na fiskalnu kasu ako je plaćanje gotovinom
        // Za sada samo beležimo prodaju usluge
    }

    // Ažuriraj status servisa
    now := time.Now()
    _, err = tx.Exec(`UPDATE service_orders SET status = $1, completed_at = $2, updated_at = $3 WHERE id = $4`,
        StatusCompleted, now, now, orderID)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

Http handler za kreiranje prodaje sa fiskalnom integracijom je prikazan u nastavku.

```go
// =============================================
// handlers/sale_handler.go
// =============================================
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type SaleHandler struct {
    SaleService    *services.SaleService
    FiscalClient   *services.FiscalAPIClient
    WarehouseService *services.WarehouseService
}

type CreateSaleRequest struct {
    CustomerID    *uuid.UUID           `json:"customer_id"`
    PaymentMethod models.PaymentMethod `json:"payment_method"`
    Items         []SaleItemRequest    `json:"items"`
}

type SaleItemRequest struct {
    ProductID uuid.UUID `json:"product_id"`
    Quantity  int       `json:"quantity"`
}

func (h *SaleHandler) CreateSale(c *gin.Context) {
    var req CreateSaleRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 1. Izračunaj total
    var totalNoVat, totalVat, totalWithVat float64
    var fiscalItems []services.FiscalReceiptItem

    for _, item := range req.Items {
        // Dohvati proizvod iz baze
        product, err := h.SaleService.GetProduct(item.ProductID)
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
            return
        }

        itemTotalNoVat := product.SellingPriceNoVat() * float64(item.Quantity)
        itemVat
```

