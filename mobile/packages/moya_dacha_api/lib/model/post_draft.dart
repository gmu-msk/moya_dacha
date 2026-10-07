//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class PostDraft {
  /// Returns a new [PostDraft] instance.
  PostDraft({
    this.mediaIds = const [],
    this.caption,
    this.visibility,
    this.placeId,
    this.groupIds = const [],
    this.question,
    this.visibilityGroupId,
  });

  /// Идентификаторы уже загруженных фотографий, в том порядке, в котором они должны стоять в посте. 
  List<String> mediaIds;

  /// Подпись, до 1000 символов, может быть пустой
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  String? caption;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  PostVisibility? visibility;

  /// Место поста — пункт из подсказок `GET /places`; нет, `null` или пустая строка — без места (specs/027-post-place.md) 
  String? placeId;

  /// Группы, в которых выложить пост: только те, где автор — участник; нет, `null` или пусто — ни в какой (specs/030-group-posts.md) 
  List<String>? groupIds;

  /// Пост-вопрос; нет или `false` — обычный пост. Потом не меняется (specs/033-question-posts.md, требование 1) 
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  bool? question;

  /// Пост увидят только участники этой группы; автор должен быть её участником. Тогда `visibility` не учитывается — приложение шлёт `me`, чтобы сервер без этой фичи не выложил пост всем. Группа сама добавляется к `group_ids`. Нет, `null` или пусто — видимость по `visibility` (specs/031-group-visibility.md) 
  String? visibilityGroupId;

  @override
  bool operator ==(Object other) => identical(this, other) || other is PostDraft &&
    _deepEquality.equals(other.mediaIds, mediaIds) &&
    other.caption == caption &&
    other.visibility == visibility &&
    other.placeId == placeId &&
    _deepEquality.equals(other.groupIds, groupIds) &&
    other.question == question &&
    other.visibilityGroupId == visibilityGroupId;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (mediaIds.hashCode) +
    (caption == null ? 0 : caption!.hashCode) +
    (visibility == null ? 0 : visibility!.hashCode) +
    (placeId == null ? 0 : placeId!.hashCode) +
    (groupIds == null ? 0 : groupIds!.hashCode) +
    (question == null ? 0 : question!.hashCode) +
    (visibilityGroupId == null ? 0 : visibilityGroupId!.hashCode);

  @override
  String toString() => 'PostDraft[mediaIds=$mediaIds, caption=$caption, visibility=$visibility, placeId=$placeId, groupIds=$groupIds, question=$question, visibilityGroupId=$visibilityGroupId]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'media_ids'] = this.mediaIds;
    if (this.caption != null) {
      json[r'caption'] = this.caption;
    } else {
      json[r'caption'] = null;
    }
    if (this.visibility != null) {
      json[r'visibility'] = this.visibility;
    } else {
      json[r'visibility'] = null;
    }
    if (this.placeId != null) {
      json[r'place_id'] = this.placeId;
    } else {
      json[r'place_id'] = null;
    }
    if (this.groupIds != null) {
      json[r'group_ids'] = this.groupIds;
    } else {
      json[r'group_ids'] = null;
    }
    if (this.question != null) {
      json[r'question'] = this.question;
    } else {
      json[r'question'] = null;
    }
    if (this.visibilityGroupId != null) {
      json[r'visibility_group_id'] = this.visibilityGroupId;
    } else {
      json[r'visibility_group_id'] = null;
    }
    return json;
  }

  /// Returns a new [PostDraft] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static PostDraft? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'media_ids'), 'Required key "PostDraft[media_ids]" is missing from JSON.');
        assert(json[r'media_ids'] != null, 'Required key "PostDraft[media_ids]" has a null value in JSON.');
        return true;
      }());

      return PostDraft(
        mediaIds: json[r'media_ids'] is Iterable
            ? (json[r'media_ids'] as Iterable).cast<String>().toList(growable: false)
            : const [],
        caption: mapValueOfType<String>(json, r'caption'),
        visibility: PostVisibility.fromJson(json[r'visibility']),
        placeId: mapValueOfType<String>(json, r'place_id'),
        groupIds: json[r'group_ids'] is Iterable
            ? (json[r'group_ids'] as Iterable).cast<String>().toList(growable: false)
            : const [],
        question: mapValueOfType<bool>(json, r'question'),
        visibilityGroupId: mapValueOfType<String>(json, r'visibility_group_id'),
      );
    }
    return null;
  }

  static List<PostDraft> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <PostDraft>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = PostDraft.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, PostDraft> mapFromJson(dynamic json) {
    final map = <String, PostDraft>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = PostDraft.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of PostDraft-objects as value to a dart map
  static Map<String, List<PostDraft>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<PostDraft>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = PostDraft.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'media_ids',
  };
}

