package media

const (
	// AvatarMaxBytes — сколько может весить загружаемая картинка.
	AvatarMaxBytes = 5 << 20
	// AvatarMaxSide — до какого размера уменьшается аватар.
	AvatarMaxSide = 512
	// AvatarExt — расширение, с которым аватар сохраняется.
	AvatarExt = ".jpg"
)

// NormalizeAvatar приводит загруженную картинку к виду, в котором аватар
// хранится (specs/002-profile.md, требование 7).
func NormalizeAvatar(raw []byte) ([]byte, error) {
	avatar, err := Normalize(raw, AvatarMaxSide)
	if err != nil {
		return nil, err
	}
	return avatar.Content, nil
}
