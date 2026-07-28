package config

var (
	ReadPoolSize = getInt("READ_POOL_SIZE", 4)
	DatabaseFile = getString("DATABASE_FILE", "data.db")
)
