BEGIN;

CREATE TABLE profiles (
    id text COLLATE "C" PRIMARY KEY,
    name text NOT NULL,
    bio text NOT NULL DEFAULT '',
    photo_url text NOT NULL DEFAULT '',
    active boolean NOT NULL DEFAULT true,
    movies text[] NOT NULL DEFAULT '{}',
    music text[] NOT NULL DEFAULT '{}',
    books text[] NOT NULL DEFAULT '{}',
    position integer NOT NULL UNIQUE,
    version bigint NOT NULL DEFAULT 1,
    CONSTRAINT profiles_id_check CHECK (octet_length(id) BETWEEN 1 AND 64),
    CONSTRAINT profiles_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100),
    CONSTRAINT profiles_bio_check CHECK (char_length(bio) <= 2000),
    CONSTRAINT profiles_position_check CHECK (position > 0),
    CONSTRAINT profiles_version_check CHECK (version > 0)
);

CREATE TABLE pairs (
    left_id text COLLATE "C" NOT NULL,
    right_id text COLLATE "C" NOT NULL,
    left_liked boolean NOT NULL DEFAULT false,
    right_liked boolean NOT NULL DEFAULT false,
    left_blocked boolean NOT NULL DEFAULT false,
    right_blocked boolean NOT NULL DEFAULT false,
    PRIMARY KEY (left_id, right_id),
    CONSTRAINT pairs_order_check CHECK (left_id < right_id),
    CONSTRAINT pairs_left_profile_fk FOREIGN KEY (left_id) REFERENCES profiles(id),
    CONSTRAINT pairs_right_profile_fk FOREIGN KEY (right_id) REFERENCES profiles(id)
);

CREATE TABLE matches (
    left_id text COLLATE "C" NOT NULL,
    right_id text COLLATE "C" NOT NULL,
    active boolean NOT NULL DEFAULT true,
    PRIMARY KEY (left_id, right_id),
    CONSTRAINT matches_pair_fk FOREIGN KEY (left_id, right_id) REFERENCES pairs(left_id, right_id)
);

COMMIT;
