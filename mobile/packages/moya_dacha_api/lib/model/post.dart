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

  @override
  bool operator ==(Object other) => identical(this, other) || other is Post &&
    other.id == id &&
    other.createdAt == createdAt &&
    other.caption == caption &&
    other.author == author &&
    _deepEquality.equals(other.media, media) &&
    other.likes == likes &&
    other.liked == liked;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (createdAt.hashCode) +
    (caption.hashCode) +
    (author.hashCode) +
    (media.hashCode) +
    (likes.hashCode) +
    (liked.hashCode);

  @override
  String toString() => 'Post[id=$id, createdAt=$createdAt, caption=$caption, author=$author, media=$media, likes=$likes, liked=$liked]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
      json[r'caption'] = this.caption;
      json[r'author'] = this.author;
      json[r'media'] = this.media;
      json[r'likes'] = this.likes;
      json[r'liked'] = this.liked;
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
  };
}

