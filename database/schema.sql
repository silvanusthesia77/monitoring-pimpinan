CREATE TABLE IF NOT EXISTS users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(120) NOT NULL,
  email VARCHAR(160) NOT NULL UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  role ENUM('staf', 'pimpinan') NOT NULL,
  position VARCHAR(160) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS files (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  original_name VARCHAR(255) NOT NULL,
  stored_name VARCHAR(255) NOT NULL,
  mime_type VARCHAR(120) NOT NULL,
  size_bytes BIGINT NOT NULL,
  uploaded_by BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_files_uploaded_by FOREIGN KEY (uploaded_by) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS agendas (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  title VARCHAR(180) NOT NULL,
  location VARCHAR(180) NOT NULL,
  start_at DATETIME NOT NULL,
  end_at DATETIME NOT NULL,
  organizer VARCHAR(180) NOT NULL,
  staff_note TEXT NOT NULL,
  invitation_file_id BIGINT NULL,
  status ENUM('menunggu', 'hadir', 'diwakili') NOT NULL DEFAULT 'menunggu',
  delegate VARCHAR(180) NULL,
  leader_note TEXT NULL,
  validated_by BIGINT NULL,
  validated_at DATETIME NULL,
  pulled_back_at DATETIME NULL,
  documentation_file_id BIGINT NULL,
  report_note TEXT NULL,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_agendas_invitation_file FOREIGN KEY (invitation_file_id) REFERENCES files(id),
  CONSTRAINT fk_agendas_documentation_file FOREIGN KEY (documentation_file_id) REFERENCES files(id),
  CONSTRAINT fk_agendas_created_by FOREIGN KEY (created_by) REFERENCES users(id),
  CONSTRAINT fk_agendas_validated_by FOREIGN KEY (validated_by) REFERENCES users(id),
  INDEX idx_agendas_start_at (start_at),
  INDEX idx_agendas_status (status)
);

CREATE TABLE IF NOT EXISTS notifications (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  audience ENUM('staf', 'pimpinan', 'semua') NOT NULL,
  title VARCHAR(180) NOT NULL,
  body TEXT NOT NULL,
  email_sent BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_notifications_audience_created (audience, created_at)
);
