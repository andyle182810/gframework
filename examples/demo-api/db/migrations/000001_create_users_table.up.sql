-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create index on email for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Create index on created_at for pagination
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);

-- Insert sample data
INSERT INTO users (id, name, email, created_at, updated_at) VALUES
    ('550e8400-e29b-41d4-a716-446655440000', 'John Doe', 'john@example.com', NOW(), NOW()),
    ('550e8400-e29b-41d4-a716-446655440001', 'Jane Smith', 'jane@example.com', NOW(), NOW()),
    ('550e8400-e29b-41d4-a716-446655440002', 'Bob Johnson', 'bob@example.com', NOW(), NOW())
ON CONFLICT (email) DO NOTHING;
