//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Comment {
  /// Returns a new [Comment] instance.
  Comment({
    required this.id,
    required this.createdAt,
    required this.text,
    required this.author,
  });

  /// Идентификатор комментария (UUID)
  String id;

  /// Когда комментарий оставлен
  DateTime createdAt;

  /// Текст комментария
  String text;

  Author author;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Comment &&
    other.id == id &&
    other.createdAt == createdAt &&
    other.text == text &&
    other.author == author;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (createdAt.hashCode) +
    (text.hashCode) +
    (author.hashCode);

  @override
  String toString() => 'Comment[id=$id, createdAt=$createdAt, text=$text, author=$author]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
      json[r'text'] = this.text;
      json[r'author'] = this.author;
    return json;
  }

  /// Returns a new [Comment] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Comment? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "Comment[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "Comment[id]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "Comment[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "Comment[created_at]" has a null value in JSON.');
        assert(json.containsKey(r'text'), 'Required key "Comment[text]" is missing from JSON.');
        assert(json[r'text'] != null, 'Required key "Comment[text]" has a null value in JSON.');
        assert(json.containsKey(r'author'), 'Required key "Comment[author]" is missing from JSON.');
        assert(json[r'author'] != null, 'Required key "Comment[author]" has a null value in JSON.');
        return true;
      }());

      return Comment(
        id: mapValueOfType<String>(json, r'id')!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
        text: mapValueOfType<String>(json, r'text')!,
        author: Author.fromJson(json[r'author'])!,
      );
    }
    return null;
  }

  static List<Comment> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Comment>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Comment.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Comment> mapFromJson(dynamic json) {
    final map = <String, Comment>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Comment.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Comment-objects as value to a dart map
  static Map<String, List<Comment>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Comment>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Comment.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'created_at',
    'text',
    'author',
  };
}

