-- Nabavke — storniranje umesto fizičkog brisanja (računovodstveni tretman ulaznih dokumenata)
ALTER TABLE nabavke ADD COLUMN stornirano INTEGER NOT NULL DEFAULT 0;
ALTER TABLE nabavke ADD COLUMN razlog_storniranja TEXT;
