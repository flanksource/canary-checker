package db

func FetchConfig(configType, name string) (string, error) {
	var config string
	err := Gorm.Table("config_item").Select("config").Where("name = ? AND config_type = ?", name, configType).
		Find(&config).Error
	return config, err
}
