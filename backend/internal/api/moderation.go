// Владелец удаляет чужое из дашборда: specs/023-moderation.md, ADR-0026.
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// mountModeration вешает действия раздела «Модерация». Все они — DELETE:
// чужая страница не пошлёт такой запрос без разрешения CORS, а его сервис
// не даёт, так что запомненный браузером пароль ею не воспользуется
// (требование 2).
func (s *Server) mountModeration(mux *http.ServeMux) {
	for pattern, handle := range map[string]func(context.Context, string) (bool, error){
		"DELETE " + dashboardPath + "/posts/{id}":            s.moderatePost,
		"DELETE " + dashboardPath + "/comments/{id}":         s.moderateComment,
		"DELETE " + dashboardPath + "/posts/{id}/reports":    s.keepPost,
		"DELETE " + dashboardPath + "/comments/{id}/reports": s.keepComment,
	} {
		mux.Handle(pattern, s.dashboardAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			found := false
			var err error
			// Не UUID — это «такого нет», а не ошибка разбора в базе
			// (требование 9).
			if isUUID(id) {
				found, err = handle(r.Context(), id)
			}
			switch {
			case err != nil:
				internalError(w, r, err)
			case !found:
				http.NotFound(w, r)
			default:
				w.WriteHeader(http.StatusNoContent)
			}
		})))
	}

	// Убрать человека из геогруппы (specs/029-groups.md, требование 31).
	mux.Handle("DELETE "+dashboardPath+"/groups/{group}/members/{user}", s.dashboardAuth(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			group, user := r.PathValue("group"), r.PathValue("user")
			found := false
			var err error
			if isUUID(group) && isUUID(user) {
				found, err = s.removeFromPlaceGroup(r.Context(), group, user)
			}
			switch {
			case err != nil:
				internalError(w, r, err)
			case !found:
				http.NotFound(w, r)
			default:
				w.WriteHeader(http.StatusNoContent)
			}
		})))
}

// removeFromPlaceGroup убирает участника или приглашение из геогруппы.
// Группы по интересам — дело их хозяина, отсюда их не трогают.
func (s *Server) removeFromPlaceGroup(ctx context.Context, group, user string) (bool, error) {
	tag, err := s.db.Exec(ctx, `
		DELETE FROM group_members gm USING groups g
		WHERE g.id = gm.group_id AND g.kind = 'place' AND gm.group_id = $1 AND gm.user_id = $2`,
		group, user)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() > 0 {
		slog.Info("владелец убрал из геогруппы", "group", group, "user", user)
	}
	return tag.RowsAffected() > 0, nil
}

// moderatePost удаляет пост так же, как удалил бы его автор (007):
// с фото в хранилище, а лайки, комментарии, жалобы и уведомления уходят
// каскадом (требование 7). Автора не извещают (требование 10).
func (s *Server) moderatePost(ctx context.Context, id string) (bool, error) {
	var author string
	err := s.db.QueryRow(ctx, `SELECT author_id FROM posts WHERE id = $1`, id).Scan(&author)
	if err != nil {
		return false, ignoreNoRows(err)
	}
	keys, err := s.removePost(ctx, id)
	if err != nil {
		return false, err
	}
	s.deleteFiles(ctx, keys)
	slog.Info("владелец удалил пост", "post", id, "author", author)
	return true, nil
}

// moderateComment удаляет комментарий, чей бы он ни был (требование 8).
func (s *Server) moderateComment(ctx context.Context, id string) (bool, error) {
	var author string
	err := s.db.QueryRow(ctx,
		`DELETE FROM comments WHERE id = $1 RETURNING author_id`, id).Scan(&author)
	if err != nil {
		return false, ignoreNoRows(err)
	}
	slog.Info("владелец удалил комментарий", "comment", id, "author", author)
	return true, nil
}

// keepPost — «Оставить»: жалобы на пост стираются, пост остаётся.
// Жалобы на комментарии под ним не трогаются (требование 11).
func (s *Server) keepPost(ctx context.Context, id string) (bool, error) {
	return s.dropReports(ctx, `posts`, `post_id`, id)
}

func (s *Server) keepComment(ctx context.Context, id string) (bool, error) {
	return s.dropReports(ctx, `comments`, `comment_id`, id)
}

// dropReports стирает жалобы на существующий пост или комментарий.
// table и column — константы из кода выше, не ввод человека.
func (s *Server) dropReports(ctx context.Context, table, column, id string) (bool, error) {
	var found bool
	err := s.db.QueryRow(ctx, `
		WITH target AS (SELECT id FROM `+table+` WHERE id = $1),
		     gone AS (DELETE FROM reports WHERE `+column+` IN (SELECT id FROM target))
		SELECT EXISTS (SELECT 1 FROM target)`, id).Scan(&found)
	return found, err
}

// ignoreNoRows превращает «строки нет» в «не нашлось» без ошибки.
func ignoreNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}
