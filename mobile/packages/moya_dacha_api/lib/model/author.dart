//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Author {
  /// Returns a new [Author] instance.
  Author({
    required this.id,
    required this.name,
    this.avatarUrl,
  });

  /// Идентификатор пользователя (UUID)
  String id;

  /// Отображаемое имя
  String name;

  /// Ссылка на аватар или `null`, если аватара нет. Может быть относительной — клиент достраивает её до адреса сервиса. 
  String? avatarUrl;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Author &&
    other.id == id &&
    other.name == name &&
    other.avatarUrl == avatarUrl;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (name.hashCode) +
    (avatarUrl == null ? 0 : avatarUrl!.hashCode);

  @override
  String toString() => 'Author[id=$id, name=$name, avatarUrl=$avatarUrl]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'name'] = this.name;
    if (this.avatarUrl != null) {
      json[r'avatar_url'] = this.avatarUrl;
    } else {
      json[r'avatar_url'] = null;
    }
    return json;
  }

  /// Returns a new [Author] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Author? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "Author[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "Author[id]" has a null value in JSON.');
        assert(json.containsKey(r'name'), 'Required key "Author[name]" is missing from JSON.');
        assert(json[r'name'] != null, 'Required key "Author[name]" has a null value in JSON.');
        return true;
      }());

      return Author(
        id: mapValueOfType<String>(json, r'id')!,
        name: mapValueOfType<String>(json, r'name')!,
        avatarUrl: mapValueOfType<String>(json, r'avatar_url'),
      );
    }
    return null;
  }

  static List<Author> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Author>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Author.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Author> mapFromJson(dynamic json) {
    final map = <String, Author>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Author.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Author-objects as value to a dart map
  static Map<String, List<Author>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Author>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Author.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'name',
  };
}

