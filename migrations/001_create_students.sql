CREATE TABLE IF NOT EXISTS students ( 
    id         SERIAL       PRIMARY KEY, 
    name       VARCHAR(50)  NOT NULL, 
    NIM        VARCHAR(25) NOT NULL UNIQUE, 
    Grade      FLOAT NOT NULL, 
    is_active  BOOLEAN      NOT NULL DEFAULT TRUE, 
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW() 
); 
  -- Keunikan username tanpa membedakan huruf besar dan kecil. -- Inilah yang menggantikan pemeriksaan manual di pertemuan 2. 
CREATE UNIQUE INDEX IF NOT EXISTS student_name_lower_key 
    ON students (LOWER(name));