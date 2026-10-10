# Dokumentasi REST API SimpleNext (Untuk Frontend Developer)

Dokumentasi ini disusun khusus bagi tim **Frontend (Web/Mobile)** untuk mengintegrasikan antarmuka pengguna (*UI/UX*) dengan backend **SimpleNext**.

---

## 1. Ringkasan & Konvensi API

### 1.1 Base URL & Environment
| Environment | Base URL |
| :--- | :--- |
| **Local Development** | `http://localhost:8080` |
| **Production** | *(Sesuai domain deployment)* |

Semua endpoint bisnis berada di bawah prefix:  
`/api/v1`

---

### 1.2 Headers Standar
Setiap permintaan HTTP harus menyertakan header berikut:
```http
Content-Type: application/json
Accept: application/json
```

---

### 1.3 Format Respon (Envelope Pattern)

Backend SimpleNext mengembalikan format JSON konsisten:

#### Respon Berhasil (Success)
Respon dapat berupa objek dengan key `data`, atau disertai `message` informatif:
```json
{
  "message": "Draft Journal Entry berhasil dibuat",
  "data": { ... }
}
```

#### Respon Gagal (Error)
Jika terjadi kesalahan validasi atau sistem (HTTP Status `4xx` atau `5xx`):
```json
{
  "error": "Pesan deskripsi kesalahan yang informatif"
}
```

---

### 1.4 Siklus Status Dokumen (`DocStatus`)
Sesuai standar ERP modern, dokumen transaksi (seperti **Journal Entry**) mengikuti siklus hidup (*document lifecycle*) integer berikut:

| Value | Nama Status | Keterangan & State UI | Aksi yang Diperbolehkan |
| :---: | :--- | :--- | :--- |
| `0` | **Draft** | Dokumen konsep. Belum berpengaruh ke Buku Besar (General Ledger). Form dapat diedit. | Tombol **Edit**, Tombol **Submit** |
| `1` | **Submitted** | Dokumen resmi terkunci (*read-only*). Transaksi telah diposting ke Buku Besar. | Tombol **Cancel** |
| `2` | **Cancelled** | Dokumen dibatalkan (*read-only*). Sistem otomatis menerbitkan entri pembalik (*reversal*) di Buku Besar. | Tidak ada (Arsip riwayat) |

---

## 2. Definisi Tipe TypeScript (Siap Pakai untuk Frontend)

Frontend developer dapat menyalin (*copy-paste*) definisi tipe data berikut ke dalam proyek TypeScript (misal: `src/types/api.ts`):

```typescript
// ==========================================
// 1. Shared Types & Status
// ==========================================
export type DocStatus = 0 | 1 | 2;

export const DocStatusEnum = {
  DRAFT: 0,
  SUBMITTED: 1,
  CANCELLED: 2,
} as const;

export interface ApiResponse<T> {
  message?: string;
  data: T;
}

export interface ApiErrorResponse {
  error: string;
}

// ==========================================
// 2. Chart of Accounts (Bagan Akun)
// ==========================================
export type AccountRootType = 'Asset' | 'Liability' | 'Equity' | 'Income' | 'Expense';

export interface Account {
  name: string;           // Identifier unik, e.g. "1112 - Rekening Bank BCA"
  account_number: string; // e.g. "1112"
  account_name: string;   // e.g. "Rekening Bank BCA"
  account_type: string;   // e.g. "Bank", "Cash", "Receivable", "Payable"
  root_type: AccountRootType;
  is_group: boolean;      // true jika akun induk (tidak boleh dipilih pada transaksi)
  parent_account: string | null;
  created_at: string;     // ISO 8601 Timestamp
  children?: Account[];   // Diisi jika query ?tree=true
}

// ==========================================
// 3. Journal Entry (Voucher Jurnal)
// ==========================================
export interface JournalEntryAccountItem {
  id: string;
  parent: string;
  idx: number;
  account: string;        // ID akun, e.g. "1112 - Rekening Bank BCA"
  debit: number;
  credit: number;
  user_remark: string;
}

export interface JournalEntry {
  name: string;           // Nomor Voucher, e.g. "JV-2026-00001"
  posting_date: string;   // Format ISO / Date YYYY-MM-DD
  docstatus: DocStatus;
  total_debit: number;
  total_credit: number;
  user_remark: string;
  accounts: JournalEntryAccountItem[];
  created_at: string;
  updated_at: string;
}

// Payload untuk membuat Journal Entry baru
export interface CreateJournalEntryItemPayload {
  account: string;        // Wajib akun non-grup (is_group: false)
  debit: number;
  credit: number;
  user_remark?: string;
}

export interface CreateJournalEntryPayload {
  posting_date?: string;  // Format YYYY-MM-DD (jika kosong, otomatis hari ini)
  user_remark?: string;
  accounts: CreateJournalEntryItemPayload[]; // Minimal 2 baris akun
}

// ==========================================
// 4. Reports (Laporan Keuangan)
// ==========================================
export interface GeneralLedgerEntry {
  id: string;
  posting_date: string;
  account: string;
  debit: number;
  credit: number;
  voucher_type: string;   // e.g. "Journal Entry"
  voucher_no: string;     // e.g. "JV-2026-00001"
  is_cancelled: boolean;
  remarks: string;
  created_at: string;
}

export interface TrialBalanceRow {
  account: string;
  account_name: string;
  root_type: AccountRootType;
  total_debit: number;
  total_credit: number;
  net_balance: number;
}

export interface TrialBalanceReport {
  data: TrialBalanceRow[];
  total_debit: number;
  total_credit: number;
  is_balanced: boolean;
}
```

---

## 3. Katalog Lengkap Endpoint

### 3.1 Health Check

#### `GET /health`
Mengecek status ketersediaan backend server.

- **Request:**
  ```http
  GET /health HTTP/1.1
  Host: localhost:8080
  ```
- **Response `200 OK`:**
  ```json
  {
    "status": "ok",
    "app": "simplenext"
  }
  ```

---

### 3.2 Modul Chart of Accounts (Bagan Akun)

#### `GET /api/v1/accounts`
Mengambil seluruh daftar akun dalam format **daftar datar (*flat array*)**.  
*Cocok untuk pencarian autocomplete sederhana atau tabel data.*

- **Query Parameters:** Tidak ada.
- **Response `200 OK`:**
  ```json
  {
    "data": [
      {
        "name": "1000 - Aset",
        "account_number": "1000",
        "account_name": "Aset",
        "account_type": "Asset",
        "root_type": "Asset",
        "is_group": true,
        "parent_account": null,
        "created_at": "2026-10-10T10:00:00Z"
      },
      {
        "name": "1112 - Rekening Bank BCA",
        "account_number": "1112",
        "account_name": "Rekening Bank BCA",
        "account_type": "Bank",
        "root_type": "Asset",
        "is_group": false,
        "parent_account": "1110 - Kas dan Bank",
        "created_at": "2026-10-10T10:00:00Z"
      }
    ]
  }
  ```

---

#### `GET /api/v1/accounts?tree=true`
Mengambil daftar akun dalam struktur **pohon hierarki (*tree structure*)** bersarang berdasarkan `parent_account`.  
*Sangat direkomendasikan untuk komponen Tree View, Sidebar Bagan Akun, atau Cascader Select di UI.*

- **Query Parameters:**
  | Parameter | Tipe | Wajib | Deskripsi |
  | :--- | :--- | :---: | :--- |
  | `tree` | `boolean` | Ya (`true`) | Mengaktifkan pengelompokan hierarki bertingkat. |

- **Response `200 OK`:**
  ```json
  {
    "data": [
      {
        "name": "1000 - Aset",
        "account_number": "1000",
        "account_name": "Aset",
        "account_type": "Asset",
        "root_type": "Asset",
        "is_group": true,
        "parent_account": null,
        "created_at": "2026-10-10T10:00:00Z",
        "children": [
          {
            "name": "1100 - Aset Lancar",
            "account_number": "1100",
            "account_name": "Aset Lancar",
            "account_type": "Asset",
            "root_type": "Asset",
            "is_group": true,
            "parent_account": "1000 - Aset",
            "created_at": "2026-10-10T10:00:00Z",
            "children": [
              {
                "name": "1110 - Kas dan Bank",
                "account_number": "1110",
                "account_name": "Kas dan Bank",
                "account_type": "Asset",
                "root_type": "Asset",
                "is_group": true,
                "parent_account": "1100 - Aset Lancar",
                "created_at": "2026-10-10T10:00:00Z",
                "children": [
                  {
                    "name": "1111 - Kas Kecil",
                    "account_number": "1111",
                    "account_name": "Kas Kecil",
                    "account_type": "Cash",
                    "root_type": "Asset",
                    "is_group": false,
                    "parent_account": "1110 - Kas dan Bank",
                    "created_at": "2026-10-10T10:00:00Z"
                  },
                  {
                    "name": "1112 - Rekening Bank BCA",
                    "account_number": "1112",
                    "account_name": "Rekening Bank BCA",
                    "account_type": "Bank",
                    "root_type": "Asset",
                    "is_group": false,
                    "parent_account": "1110 - Kas dan Bank",
                    "created_at": "2026-10-10T10:00:00Z"
                  }
                ]
              }
            ]
          }
        ]
      }
    ]
  }
  ```

---

### 3.3 Modul Journal Entry (Transaksi Jurnal Umum)

#### `POST /api/v1/journal-entries`
Membuat dokumen konsep baru (*Draft Journal Entry*). Dokumen yang berhasil dibuat berstatus `docstatus = 0`.

- **Request Body Parameters:**
  | Field | Tipe | Wajib | Keterangan |
  | :--- | :--- | :---: | :--- |
  | `posting_date` | `string` | Tidak | Format `YYYY-MM-DD`. Default tanggal hari ini. |
  | `user_remark` | `string` | Tidak | Catatan memo transaksi. |
  | `accounts` | `array` | **Ya** | Daftar baris akun. **Minimal 2 baris**. |
  | `accounts[].account` | `string` | **Ya** | Nama akun leaf (Wajib `is_group == false`). |
  | `accounts[].debit` | `number` | **Ya** | Nilai nominal debit (>= 0). |
  | `accounts[].credit` | `number` | **Ya** | Nilai nominal kredit (>= 0). |
  | `accounts[].user_remark`| `string` | Tidak | Catatan per baris akun. |

- **Aturan Validasi Form:**
  1. `accounts` minimal 2 item.
  2. Akun tidak boleh berupa akun grup (`is_group: false`).
  3. Salah satu dari `debit` atau `credit` harus bernilai `> 0`.
  4. Tidak boleh mengisi debit dan kredit sekaligus pada baris yang sama.
  5. `Total Debit` harus **tepat sama** dengan `Total Kredit`.

- **Contoh Request Body:**
  ```json
  {
    "posting_date": "2026-10-10",
    "user_remark": "Pembayaran sewa gedung operasional bulan Oktober",
    "accounts": [
      {
        "account": "5120 - Beban Sewa",
        "debit": 5000000,
        "credit": 0,
        "user_remark": "Sewa kantor"
      },
      {
        "account": "1112 - Rekening Bank BCA",
        "debit": 0,
        "credit": 5000000,
        "user_remark": "Transfer via BCA"
      }
    ]
  }
  ```

- **Response `201 Created`:**
  ```json
  {
    "message": "Draft Journal Entry berhasil dibuat",
    "data": {
      "name": "JV-2026-00001",
      "posting_date": "2026-10-10T00:00:00Z",
      "docstatus": 0,
      "total_debit": 5000000,
      "total_credit": 5000000,
      "user_remark": "Pembayaran sewa gedung operasional bulan Oktober",
      "accounts": [
        {
          "id": "e305e510-444a-4710-be02-bcf5ef2aa9a1",
          "parent": "JV-2026-00001",
          "idx": 1,
          "account": "5120 - Beban Sewa",
          "debit": 5000000,
          "credit": 0,
          "user_remark": "Sewa kantor"
        },
        {
          "id": "c1f7b9f2-0a12-42fe-b582-b7e651ba5c90",
          "parent": "JV-2026-00001",
          "idx": 2,
          "account": "1112 - Rekening Bank BCA",
          "debit": 0,
          "credit": 5000000,
          "user_remark": "Transfer via BCA"
        }
      ],
      "created_at": "2026-10-10T14:30:00Z",
      "updated_at": "2026-10-10T14:30:00Z"
    }
  }
  ```

- **Contoh Error Response `400 Bad Request`:**
  ```json
  {
    "error": "jurnal tidak seimbang: Total Debit (5000000.00) != Total Kredit (4000000.00)"
  }
  ```
  *atau jika memilih akun grup:*
  ```json
  {
    "error": "transaksi tidak dapat dilakukan pada akun grup/induk ('1110 - Kas dan Bank')"
  }
  ```

---

#### `GET /api/v1/journal-entries/{name}`
Mengambil detail satu dokumen Journal Entry beserta daftar rincian baris akunnya.

- **Path Parameters:**
  | Parameter | Tipe | Wajib | Deskripsi |
  | :--- | :--- | :---: | :--- |
  | `name` | `string` | Ya | Nomor voucher jurnal (contoh: `JV-2026-00001`). |

- **Response `200 OK`:**
  ```json
  {
    "data": {
      "name": "JV-2026-00001",
      "posting_date": "2026-10-10T00:00:00Z",
      "docstatus": 0,
      "total_debit": 5000000,
      "total_credit": 5000000,
      "user_remark": "Pembayaran sewa gedung operasional bulan Oktober",
      "accounts": [
        {
          "id": "e305e510-444a-4710-be02-bcf5ef2aa9a1",
          "parent": "JV-2026-00001",
          "idx": 1,
          "account": "5120 - Beban Sewa",
          "debit": 5000000,
          "credit": 0,
          "user_remark": "Sewa kantor"
        },
        {
          "id": "c1f7b9f2-0a12-42fe-b582-b7e651ba5c90",
          "parent": "JV-2026-00001",
          "idx": 2,
          "account": "1112 - Rekening Bank BCA",
          "debit": 0,
          "credit": 5000000,
          "user_remark": "Transfer via BCA"
        }
      ],
      "created_at": "2026-10-10T14:30:00Z",
      "updated_at": "2026-10-10T14:30:00Z"
    }
  }
  ```

- **Error Response `404 Not Found`:**
  ```json
  {
    "error": "dokumen 'JV-2026-99999' tidak ditemukan"
  }
  ```

---

#### `POST /api/v1/journal-entries/{name}/submit`
Memposting dokumen voucher jurnal berstatus `Draft (0)` menjadi `Submitted (1)`.  
Backend secara otomatis dan atomik akan:
1. Memvalidasi status dokumen harus `0` (Draft).
2. Mengubah status voucher menjadi `1` (Submitted).
3. Menerbitkan baris permanen ke Buku Besar (`gl_entries`).

- **Path Parameters:**
  | Parameter | Tipe | Wajib | Deskripsi |
  | :--- | :--- | :---: | :--- |
  | `name` | `string` | Ya | Nomor voucher jurnal (contoh: `JV-2026-00001`). |

- **Request Body:** Tidak ada (kosong).
- **Response `200 OK`:**
  ```json
  {
    "message": "Journal Entry berhasil di-Submit dan diposting ke Buku Besar (GL)",
    "data": {
      "name": "JV-2026-00001",
      "posting_date": "2026-10-10T00:00:00Z",
      "docstatus": 1,
      "total_debit": 5000000,
      "total_credit": 5000000,
      "user_remark": "Pembayaran sewa gedung operasional bulan Oktober",
      "accounts": [ ... ],
      "created_at": "2026-10-10T14:30:00Z",
      "updated_at": "2026-10-10T14:35:00Z"
    }
  }
  ```

- **Contoh Error Response `400 Bad Request`:**
  ```json
  {
    "error": "hanya dokumen dengan status Draft (0) yang dapat di-submit"
  }
  ```

---

#### `POST /api/v1/journal-entries/{name}/cancel`
Membatalkan dokumen voucher jurnal yang telah berstatus `Submitted (1)`.  
Backend secara otomatis dan atomik akan:
1. Memvalidasi status dokumen harus `1` (Submitted).
2. Mengubah status voucher menjadi `2` (Cancelled).
3. Menandai entri lama di GL sebagai `is_cancelled = true`.
4. Menerbitkan entri pembalik (*reversal entry*) di Buku Besar (`gl_entries`) dengan nilai debit & kredit yang saling bertukar secara akurat.

- **Path Parameters:**
  | Parameter | Tipe | Wajib | Deskripsi |
  | :--- | :--- | :---: | :--- |
  | `name` | `string` | Ya | Nomor voucher jurnal (contoh: `JV-2026-00001`). |

- **Request Body:** Tidak ada (kosong).
- **Response `200 OK`:**
  ```json
  {
    "message": "Journal Entry berhasil di-Cancel dan jurnal balik (reversal) telah diposting",
    "data": {
      "name": "JV-2026-00001",
      "posting_date": "2026-10-10T00:00:00Z",
      "docstatus": 2,
      "total_debit": 5000000,
      "total_credit": 5000000,
      "user_remark": "Pembayaran sewa gedung operasional bulan Oktober",
      "accounts": [ ... ],
      "created_at": "2026-10-10T14:30:00Z",
      "updated_at": "2026-10-10T14:40:00Z"
    }
  }
  ```

- **Contoh Error Response `400 Bad Request`:**
  ```json
  {
    "error": "hanya dokumen dengan status Submitted (1) yang dapat di-cancel"
  }
  ```

---

### 3.4 Modul Laporan Akuntansi (Financial Reports)

#### `GET /api/v1/reports/general-ledger`
Mengambil data seluruh mutasi Buku Besar (*General Ledger*).

- **Query Parameters:**
  | Parameter | Tipe | Wajib | Deskripsi |
  | :--- | :--- | :---: | :--- |
  | `account` | `string` | Opsional | Filter mutasi hanya untuk akun tertentu (contoh: `1112 - Rekening Bank BCA`). Jika dikosongkan, menampilkan seluruh akun. |

- **Contoh Request:**
  `GET /api/v1/reports/general-ledger?account=1112%20-%20Rekening%20Bank%20BCA`

- **Response `200 OK`:**
  ```json
  {
    "data": [
      {
        "id": "7f130b92-7471-46ab-a077-ca8bbcf9a3e2",
        "posting_date": "2026-10-10T00:00:00Z",
        "account": "1112 - Rekening Bank BCA",
        "debit": 0,
        "credit": 5000000,
        "voucher_type": "Journal Entry",
        "voucher_no": "JV-2026-00001",
        "is_cancelled": false,
        "remarks": "Transfer via BCA",
        "created_at": "2026-10-10T14:35:00Z"
      }
    ]
  }
  ```

---

#### `GET /api/v1/reports/trial-balance`
Mengambil laporan **Neraca Saldo (*Trial Balance*)** yang merangkum akumulasi mutasi dan saldo bersih per akun.  
*Sangat berguna untuk halaman Dashboard Keuangan, Laporan Neraca Saldo, dan indikator Balance status.*

- **Query Parameters:** Tidak ada.
- **Response Fields:**
  | Field | Tipe | Deskripsi |
  | :--- | :--- | :--- |
  | `data` | `array` | Rincian saldo per akun leaf. |
  | `total_debit` | `number` | Akumulasi total saldo debit seluruh akun. |
  | `total_credit` | `number` | Akumulasi total saldo kredit seluruh akun. |
  | `is_balanced` | `boolean` | `true` jika `total_debit == total_credit` (Indikator hijau di UI). |

- **Response `200 OK`:**
  ```json
  {
    "data": [
      {
        "account": "1112 - Rekening Bank BCA",
        "account_name": "Rekening Bank BCA",
        "root_type": "Asset",
        "total_debit": 10000000,
        "total_credit": 5000000,
        "net_balance": 5000000
      },
      {
        "account": "3100 - Modal Disetor",
        "account_name": "Modal Disetor",
        "root_type": "Equity",
        "total_debit": 0,
        "total_credit": 10000000,
        "net_balance": 10000000
      },
      {
        "account": "5120 - Beban Sewa",
        "account_name": "Beban Sewa",
        "root_type": "Expense",
        "total_debit": 5000000,
        "total_credit": 0,
        "net_balance": 5000000
      }
    ],
    "total_debit": 15000000,
    "total_credit": 15000000,
    "is_balanced": true
  }
  ```

> [!NOTE]
> **Formula Saldo Bersih (`net_balance`):**
> - Akun bertipe **Asset** dan **Expense** (Saldo normal Debit):  
>   `net_balance = Total Debit - Total Kredit`
> - Akun bertipe **Liability**, **Equity**, dan **Income** (Saldo normal Kredit):  
>   `net_balance = Total Kredit - Total Debit`

---

## 4. Panduan & Tips Integrasi UI Frontend

### 4.1 Logika Form Journal Entry (Client-side Validation)
Sebelum mengirim payload ke endpoint `POST /api/v1/journal-entries`, pastikan form di frontend melakukan pengecekan berikut:
1. **Dropdown Pemilihan Akun**: Filter hanya akun dengan `is_group === false`. Akun grup/induk harus diberi atribut `disabled` agar pengguna tidak salah memilih akun induk.
2. **Hitung Real-time Total Debit vs Kredit**:
   ```typescript
   const totalDebit = items.reduce((sum, item) => sum + (Number(item.debit) || 0), 0);
   const totalCredit = items.reduce((sum, item) => sum + (Number(item.credit) || 0), 0);
   const isBalanced = Math.abs(totalDebit - totalCredit) < 0.0001 && totalDebit > 0;
   ```
3. **Kunci Tombol Submit**: Nonaktifkan (*disable*) tombol simpan jika `!isBalanced` atau jumlah baris akun `< 2`.

---

### 4.2 Manajemen State Tombol Aksi Transaksi Berdasarkan `docstatus`

```typescript
function renderActionButtons(docstatus: DocStatus) {
  switch (docstatus) {
    case 0: // Draft
      return (
        <>
          <button onClick={handleSaveDraft}>Simpan Perubahan</button>
          <button className="btn-primary" onClick={handleSubmit}>Submit Dokumen</button>
        </>
      );
    case 1: // Submitted
      return (
        <>
          <span className="badge badge-success">SUBMITTED</span>
          <button className="btn-danger" onClick={handleCancel}>Batalkan (Cancel)</button>
        </>
      );
    case 2: // Cancelled
      return (
        <span className="badge badge-danger">CANCELLED</span>
      );
  }
}
```

---

### 4.3 Contoh Service API Frontend (Fetch / Axios)

Berikut contoh implementasi modul service untuk Frontend (misalnya di Next.js / Vite / React / Vue):

```typescript
// src/services/accounting.ts
import { 
  Account, 
  JournalEntry, 
  CreateJournalEntryPayload, 
  TrialBalanceReport, 
  GeneralLedgerEntry 
} from '../types/api';

const BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

async function fetchJson<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${url}`, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  });

  const body = await res.json();
  if (!res.ok) {
    throw new Error(body.error || 'Terjadi kesalahan pada server');
  }
  return body;
}

export const accountingApi = {
  // Ambil struktur pohon akun
  getAccountTree: () => 
    fetchJson<{ data: Account[] }>('/api/v1/accounts?tree=true'),

  // Buat draft voucher jurnal
  createJournalEntry: (payload: CreateJournalEntryPayload) =>
    fetchJson<{ message: string; data: JournalEntry }>('/api/v1/journal-entries', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  // Ambil detail voucher jurnal
  getJournalEntry: (name: string) =>
    fetchJson<{ data: JournalEntry }>(`/api/v1/journal-entries/${encodeURIComponent(name)}`),

  // Submit voucher jurnal
  submitJournalEntry: (name: string) =>
    fetchJson<{ message: string; data: JournalEntry }>(`/api/v1/journal-entries/${encodeURIComponent(name)}/submit`, {
      method: 'POST',
    }),

  // Cancel voucher jurnal
  cancelJournalEntry: (name: string) =>
    fetchJson<{ message: string; data: JournalEntry }>(`/api/v1/journal-entries/${encodeURIComponent(name)}/cancel`, {
      method: 'POST',
    }),

  // Laporan Neraca Saldo
  getTrialBalance: () =>
    fetchJson<TrialBalanceReport>('/api/v1/reports/trial-balance'),

  // Laporan Buku Besar
  getGeneralLedger: (account?: string) => {
    const query = account ? `?account=${encodeURIComponent(account)}` : '';
    return fetchJson<{ data: GeneralLedgerEntry[] }>(`/api/v1/reports/general-ledger${query}`);
  },
};
```
