-- Seeding Chart of Accounts (Bagan Akun Standar)
-- Root & Group Accounts
INSERT INTO accounts (name, account_number, account_name, account_type, root_type, is_group, parent_account)
VALUES
  -- 1. ASSETS
  ('1000 - Aset', '1000', 'Aset', 'Asset', 'Asset', TRUE, NULL),
  ('1100 - Aset Lancar', '1100', 'Aset Lancar', 'Asset', 'Asset', TRUE, '1000 - Aset'),
  ('1110 - Kas dan Bank', '1110', 'Kas dan Bank', 'Asset', 'Asset', TRUE, '1100 - Aset Lancar'),
  ('1111 - Kas Kecil', '1111', 'Kas Kecil', 'Cash', 'Asset', FALSE, '1110 - Kas dan Bank'),
  ('1112 - Rekening Bank BCA', '1112', 'Rekening Bank BCA', 'Bank', 'Asset', FALSE, '1110 - Kas dan Bank'),
  ('1120 - Piutang Usaha', '1120', 'Piutang Usaha', 'Receivable', 'Asset', FALSE, '1100 - Aset Lancar'),

  -- 2. LIABILITIES
  ('2000 - Kewajiban', '2000', 'Kewajiban', 'Liability', 'Liability', TRUE, NULL),
  ('2100 - Kewajiban Lancar', '2100', 'Kewajiban Lancar', 'Liability', 'Liability', TRUE, '2000 - Kewajiban'),
  ('2110 - Utang Usaha', '2110', 'Utang Usaha', 'Payable', 'Liability', FALSE, '2100 - Kewajiban Lancar'),

  -- 3. EQUITY
  ('3000 - Ekuitas', '3000', 'Ekuitas', 'Equity', 'Equity', TRUE, NULL),
  ('3100 - Modal Disetor', '3100', 'Modal Disetor', 'Equity', 'Equity', FALSE, '3000 - Ekuitas'),
  ('3200 - Laba Ditahan', '3200', 'Laba Ditahan', 'Equity', 'Equity', FALSE, '3000 - Ekuitas'),

  -- 4. INCOME
  ('4000 - Pendapatan', '4000', 'Pendapatan', 'Income', 'Income', TRUE, NULL),
  ('4100 - Pendapatan Operasional', '4100', 'Pendapatan Operasional', 'Income', 'Income', TRUE, '4000 - Pendapatan'),
  ('4110 - Pendapatan Penjualan', '4110', 'Pendapatan Penjualan', 'Income', 'Income', FALSE, '4100 - Pendapatan Operasional'),
  ('4120 - Pendapatan Jasa', '4120', 'Pendapatan Jasa', 'Income', 'Income', FALSE, '4100 - Pendapatan Operasional'),

  -- 5. EXPENSES
  ('5000 - Beban', '5000', 'Beban', 'Expense', 'Expense', TRUE, NULL),
  ('5100 - Beban Operasional', '5100', 'Beban Operasional', 'Expense', 'Expense', TRUE, '5000 - Beban'),
  ('5110 - Beban Gaji', '5110', 'Beban Gaji', 'Expense', 'Expense', FALSE, '5100 - Beban Operasional'),
  ('5120 - Beban Sewa', '5120', 'Beban Sewa', 'Expense', 'Expense', FALSE, '5100 - Beban Operasional'),
  ('5130 - Beban Listrik dan Air', '5130', 'Beban Listrik dan Air', 'Expense', 'Expense', FALSE, '5100 - Beban Operasional'),
  ('5140 - Beban Perlengkapan Kantor', '5140', 'Beban Perlengkapan Kantor', 'Expense', 'Expense', FALSE, '5100 - Beban Operasional')
ON CONFLICT (name) DO NOTHING;
