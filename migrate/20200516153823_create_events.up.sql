CREATE TABLE events (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	title TEXT NOT NULL,
	description TEXT
);

CREATE TABLE attachments (
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	file_name TEXT NOT NULL,
	mime_type TEXT
);
