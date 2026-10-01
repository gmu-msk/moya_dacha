//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class TagsUpdate {
  /// Returns a new [TagsUpdate] instance.
  TagsUpdate({
    this.tags = const [],
  });

  /// Новые тэги поста целиком; пустой массив снимает все. Обязательно: тело без него — `invalid_request`. 
  List<String> tags;

  @override
  bool operator ==(Object other) => identical(this, other) || other is TagsUpdate &&
    _deepEquality.equals(other.tags, tags);

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (tags.hashCode);

  @override
  String toString() => 'TagsUpdate[tags=$tags]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'tags'] = this.tags;
    return json;
  }

  /// Returns a new [TagsUpdate] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static TagsUpdate? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        return true;
      }());

      return TagsUpdate(
        tags: json[r'tags'] is Iterable
            ? (json[r'tags'] as Iterable).cast<String>().toList(growable: false)
            : const [],
      );
    }
    return null;
  }

  static List<TagsUpdate> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <TagsUpdate>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = TagsUpdate.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, TagsUpdate> mapFromJson(dynamic json) {
    final map = <String, TagsUpdate>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = TagsUpdate.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of TagsUpdate-objects as value to a dart map
  static Map<String, List<TagsUpdate>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<TagsUpdate>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = TagsUpdate.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
  };
}

