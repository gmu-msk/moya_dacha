//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class NotificationPost {
  /// Returns a new [NotificationPost] instance.
  NotificationPost({
    required this.id,
    required this.thumbnail,
  });

  /// Идентификатор поста (UUID)
  String id;

  Media thumbnail;

  @override
  bool operator ==(Object other) => identical(this, other) || other is NotificationPost &&
    other.id == id &&
    other.thumbnail == thumbnail;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (thumbnail.hashCode);

  @override
  String toString() => 'NotificationPost[id=$id, thumbnail=$thumbnail]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'thumbnail'] = this.thumbnail;
    return json;
  }

  /// Returns a new [NotificationPost] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static NotificationPost? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "NotificationPost[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "NotificationPost[id]" has a null value in JSON.');
        assert(json.containsKey(r'thumbnail'), 'Required key "NotificationPost[thumbnail]" is missing from JSON.');
        assert(json[r'thumbnail'] != null, 'Required key "NotificationPost[thumbnail]" has a null value in JSON.');
        return true;
      }());

      return NotificationPost(
        id: mapValueOfType<String>(json, r'id')!,
        thumbnail: Media.fromJson(json[r'thumbnail'])!,
      );
    }
    return null;
  }

  static List<NotificationPost> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <NotificationPost>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = NotificationPost.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, NotificationPost> mapFromJson(dynamic json) {
    final map = <String, NotificationPost>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = NotificationPost.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of NotificationPost-objects as value to a dart map
  static Map<String, List<NotificationPost>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<NotificationPost>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = NotificationPost.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'thumbnail',
  };
}

