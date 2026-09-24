//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class FollowUser {
  /// Returns a new [FollowUser] instance.
  FollowUser({
    required this.id,
    required this.nickname,
    required this.name,
    this.avatarUrl,
    this.relation,
  });

  /// Идентификатор пользователя (UUID)
  String id;

  /// Никнейм
  String nickname;

  /// Полное имя, может быть пустым
  String name;

  /// Ссылка на аватар или `null`
  String? avatarUrl;

  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  Relation? relation;

  @override
  bool operator ==(Object other) => identical(this, other) || other is FollowUser &&
    other.id == id &&
    other.nickname == nickname &&
    other.name == name &&
    other.avatarUrl == avatarUrl &&
    other.relation == relation;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (nickname.hashCode) +
    (name.hashCode) +
    (avatarUrl == null ? 0 : avatarUrl!.hashCode) +
    (relation == null ? 0 : relation!.hashCode);

  @override
  String toString() => 'FollowUser[id=$id, nickname=$nickname, name=$name, avatarUrl=$avatarUrl, relation=$relation]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'nickname'] = this.nickname;
      json[r'name'] = this.name;
    if (this.avatarUrl != null) {
      json[r'avatar_url'] = this.avatarUrl;
    } else {
      json[r'avatar_url'] = null;
    }
    if (this.relation != null) {
      json[r'relation'] = this.relation;
    } else {
      json[r'relation'] = null;
    }
    return json;
  }

  /// Returns a new [FollowUser] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static FollowUser? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "FollowUser[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "FollowUser[id]" has a null value in JSON.');
        assert(json.containsKey(r'nickname'), 'Required key "FollowUser[nickname]" is missing from JSON.');
        assert(json[r'nickname'] != null, 'Required key "FollowUser[nickname]" has a null value in JSON.');
        assert(json.containsKey(r'name'), 'Required key "FollowUser[name]" is missing from JSON.');
        assert(json[r'name'] != null, 'Required key "FollowUser[name]" has a null value in JSON.');
        return true;
      }());

      return FollowUser(
        id: mapValueOfType<String>(json, r'id')!,
        nickname: mapValueOfType<String>(json, r'nickname')!,
        name: mapValueOfType<String>(json, r'name')!,
        avatarUrl: mapValueOfType<String>(json, r'avatar_url'),
        relation: Relation.fromJson(json[r'relation']),
      );
    }
    return null;
  }

  static List<FollowUser> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <FollowUser>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = FollowUser.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, FollowUser> mapFromJson(dynamic json) {
    final map = <String, FollowUser>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = FollowUser.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of FollowUser-objects as value to a dart map
  static Map<String, List<FollowUser>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<FollowUser>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = FollowUser.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'nickname',
    'name',
  };
}

