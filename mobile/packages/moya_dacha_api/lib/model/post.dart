//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Post {
  /// Returns a new [Post] instance.
  Post({
    required this.id,
    required this.createdAt,
    required this.caption,
    required this.author,
    this.media = const [],
    required this.likes,
    required this.liked,
    this.bookmarks,
    this.bookmarked,
    this.question,
    this.solved,
    this.answerCommentId,
    required this.comments,
    required this.visibility,
    this.editedAt,
    this.place,
    this.distanceKm,
    this.tags = const [],
    this.groups = const [],
    this.visibilityGroup,
  });

  /// Идентификатор поста (UUID)
  String id;

  /// Когда пост опубликован
  DateTime createdAt;

  /// Подпись, может быть пустой
  String caption;

  Author author;

  /// Медиа поста в порядке, в котором их прислал автор
  List<Media> media;

  /// Сколько людей отметили пост. Кто именно — не показывается.
  int likes;

  /// Отметил ли пост тот, кто спрашивает
  bool liked;

  /// Сколько людей сохранили пост в закладки. Кто именно — не показывается (specs/032-bookmarks.md, требование 6). 
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  int? bookmarks;

  /// Сохранил ли пост в закладки тот, кто спрашивает
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  bool? bookmarked;

  /// Пост-вопрос: ждёт ответа, у него есть статус «Решён» / «Не решён» (specs/033-question-posts.md) 
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  bool? question;

  /// Вопрос решён; у обычного поста всегда `false`
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  bool? solved;

  /// Комментарий, который автор вопроса отметил решением; `null` — не отмечен, нет поля — сервер без вопросов 
  String? answerCommentId;

  /// Сколько комментариев под постом
  int comments;

  PostVisibility visibility;

  /// Когда подпись последний раз меняли; нет или `null`, если не меняли (specs/022-edit-block-delete.md) 
  DateTime? editedAt;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  Place? place;

  /// Примерное расстояние от пункта смотрящего до места поста, км, округлённое; `0` — тот же пункт. Нет или `null`, если его не посчитать или пост свой (specs/027-post-place.md, требования 7–9) 
  int? distanceKm;

  /// Тэги из хэштегов подписи в порядке подписи, в нижнем регистре; нет тэгов — пустой массив (specs/028-post-tags.md) 
  List<String> tags;

  /// Группы, в которых выложен пост и которые видны смотрящему, по названию; нет — пустой массив (specs/030-group-posts.md) 
  List<GroupBrief> groups;

  /// Пост видят только участники этой группы (specs/031-group-visibility.md). Тогда `visibility` — `friends`, чтобы старые сборки разбирали ответ; приложение смотрит сюда. Нет или `null` — видимость по `visibility`. 
  GroupBrief? visibilityGroup;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Post &&
    other.id == id &&
    other.createdAt == createdAt &&
    other.caption == caption &&
    other.author == author &&
    _deepEquality.equals(other.media, media) &&
    other.likes == likes &&
    other.liked == liked &&
    other.bookmarks == bookmarks &&
    other.bookmarked == bookmarked &&
    other.question == question &&
    other.solved == solved &&
    other.answerCommentId == answerCommentId &&
    other.comments == comments &&
    other.visibility == visibility &&
    other.editedAt == editedAt &&
    other.place == place &&
    other.distanceKm == distanceKm &&
    _deepEquality.equals(other.tags, tags) &&
    _deepEquality.equals(other.groups, groups) &&
    other.visibilityGroup == visibilityGroup;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (createdAt.hashCode) +
    (caption.hashCode) +
    (author.hashCode) +
    (media.hashCode) +
    (likes.hashCode) +
    (liked.hashCode) +
    (bookmarks == null ? 0 : bookmarks!.hashCode) +
    (bookmarked == null ? 0 : bookmarked!.hashCode) +
    (question == null ? 0 : question!.hashCode) +
    (solved == null ? 0 : solved!.hashCode) +
    (answerCommentId == null ? 0 : answerCommentId!.hashCode) +
    (comments.hashCode) +
    (visibility.hashCode) +
    (editedAt == null ? 0 : editedAt!.hashCode) +
    (place == null ? 0 : place!.hashCode) +
    (distanceKm == null ? 0 : distanceKm!.hashCode) +
    (tags.hashCode) +
    (groups.hashCode) +
    (visibilityGroup == null ? 0 : visibilityGroup!.hashCode);

  @override
  String toString() => 'Post[id=$id, createdAt=$createdAt, caption=$caption, author=$author, media=$media, likes=$likes, liked=$liked, bookmarks=$bookmarks, bookmarked=$bookmarked, question=$question, solved=$solved, answerCommentId=$answerCommentId, comments=$comments, visibility=$visibility, editedAt=$editedAt, place=$place, distanceKm=$distanceKm, tags=$tags, groups=$groups, visibilityGroup=$visibilityGroup]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
      json[r'caption'] = this.caption;
      json[r'author'] = this.author;
      json[r'media'] = this.media;
      json[r'likes'] = this.likes;
      json[r'liked'] = this.liked;
    if (this.bookmarks != null) {
      json[r'bookmarks'] = this.bookmarks;
    } else {
      json[r'bookmarks'] = null;
    }
    if (this.bookmarked != null) {
      json[r'bookmarked'] = this.bookmarked;
    } else {
      json[r'bookmarked'] = null;
    }
    if (this.question != null) {
      json[r'question'] = this.question;
    } else {
      json[r'question'] = null;
    }
    if (this.solved != null) {
      json[r'solved'] = this.solved;
    } else {
      json[r'solved'] = null;
    }
    if (this.answerCommentId != null) {
      json[r'answer_comment_id'] = this.answerCommentId;
    } else {
      json[r'answer_comment_id'] = null;
    }
      json[r'comments'] = this.comments;
      json[r'visibility'] = this.visibility;
    if (this.editedAt != null) {
      json[r'edited_at'] = this.editedAt!.toUtc().toIso8601String();
    } else {
      json[r'edited_at'] = null;
    }
    if (this.place != null) {
      json[r'place'] = this.place;
    } else {
      json[r'place'] = null;
    }
    if (this.distanceKm != null) {
      json[r'distance_km'] = this.distanceKm;
    } else {
      json[r'distance_km'] = null;
    }
      json[r'tags'] = this.tags;
      json[r'groups'] = this.groups;
    if (this.visibilityGroup != null) {
      json[r'visibility_group'] = this.visibilityGroup;
    } else {
      json[r'visibility_group'] = null;
    }
    return json;
  }

  /// Returns a new [Post] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Post? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "Post[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "Post[id]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "Post[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "Post[created_at]" has a null value in JSON.');
        assert(json.containsKey(r'caption'), 'Required key "Post[caption]" is missing from JSON.');
        assert(json[r'caption'] != null, 'Required key "Post[caption]" has a null value in JSON.');
        assert(json.containsKey(r'author'), 'Required key "Post[author]" is missing from JSON.');
        assert(json[r'author'] != null, 'Required key "Post[author]" has a null value in JSON.');
        assert(json.containsKey(r'media'), 'Required key "Post[media]" is missing from JSON.');
        assert(json[r'media'] != null, 'Required key "Post[media]" has a null value in JSON.');
        assert(json.containsKey(r'likes'), 'Required key "Post[likes]" is missing from JSON.');
        assert(json[r'likes'] != null, 'Required key "Post[likes]" has a null value in JSON.');
        assert(json.containsKey(r'liked'), 'Required key "Post[liked]" is missing from JSON.');
        assert(json[r'liked'] != null, 'Required key "Post[liked]" has a null value in JSON.');
        assert(json.containsKey(r'comments'), 'Required key "Post[comments]" is missing from JSON.');
        assert(json[r'comments'] != null, 'Required key "Post[comments]" has a null value in JSON.');
        assert(json.containsKey(r'visibility'), 'Required key "Post[visibility]" is missing from JSON.');
        assert(json[r'visibility'] != null, 'Required key "Post[visibility]" has a null value in JSON.');
        return true;
      }());

      return Post(
        id: mapValueOfType<String>(json, r'id')!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
        caption: mapValueOfType<String>(json, r'caption')!,
        author: Author.fromJson(json[r'author'])!,
        media: Media.listFromJson(json[r'media']),
        likes: mapValueOfType<int>(json, r'likes')!,
        liked: mapValueOfType<bool>(json, r'liked')!,
        bookmarks: mapValueOfType<int>(json, r'bookmarks'),
        bookmarked: mapValueOfType<bool>(json, r'bookmarked'),
        question: mapValueOfType<bool>(json, r'question'),
        solved: mapValueOfType<bool>(json, r'solved'),
        answerCommentId: mapValueOfType<String>(json, r'answer_comment_id'),
        comments: mapValueOfType<int>(json, r'comments')!,
        visibility: PostVisibility.fromJson(json[r'visibility'])!,
        editedAt: mapDateTime(json, r'edited_at', r''),
        place: Place.fromJson(json[r'place']),
        distanceKm: mapValueOfType<int>(json, r'distance_km'),
        tags: json[r'tags'] is Iterable
            ? (json[r'tags'] as Iterable).cast<String>().toList(growable: false)
            : const [],
        groups: GroupBrief.listFromJson(json[r'groups']),
        visibilityGroup: GroupBrief.fromJson(json[r'visibility_group']),
      );
    }
    return null;
  }

  static List<Post> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Post>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Post.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Post> mapFromJson(dynamic json) {
    final map = <String, Post>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Post.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Post-objects as value to a dart map
  static Map<String, List<Post>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Post>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Post.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'created_at',
    'caption',
    'author',
    'media',
    'likes',
    'liked',
    'comments',
    'visibility',
  };
}

