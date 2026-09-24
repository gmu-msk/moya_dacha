//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class UnreadNotifications {
  /// Returns a new [UnreadNotifications] instance.
  UnreadNotifications({
    required this.unread,
    required this.requests,
  });

  /// Сколько строк раздела новее последнего открытия
  int unread;

  /// Сколько заявок на подписку ждут ответа
  int requests;

  @override
  bool operator ==(Object other) => identical(this, other) || other is UnreadNotifications &&
    other.unread == unread &&
    other.requests == requests;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (unread.hashCode) +
    (requests.hashCode);

  @override
  String toString() => 'UnreadNotifications[unread=$unread, requests=$requests]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'unread'] = this.unread;
      json[r'requests'] = this.requests;
    return json;
  }

  /// Returns a new [UnreadNotifications] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static UnreadNotifications? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'unread'), 'Required key "UnreadNotifications[unread]" is missing from JSON.');
        assert(json[r'unread'] != null, 'Required key "UnreadNotifications[unread]" has a null value in JSON.');
        assert(json.containsKey(r'requests'), 'Required key "UnreadNotifications[requests]" is missing from JSON.');
        assert(json[r'requests'] != null, 'Required key "UnreadNotifications[requests]" has a null value in JSON.');
        return true;
      }());

      return UnreadNotifications(
        unread: mapValueOfType<int>(json, r'unread')!,
        requests: mapValueOfType<int>(json, r'requests')!,
      );
    }
    return null;
  }

  static List<UnreadNotifications> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <UnreadNotifications>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = UnreadNotifications.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, UnreadNotifications> mapFromJson(dynamic json) {
    final map = <String, UnreadNotifications>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = UnreadNotifications.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of UnreadNotifications-objects as value to a dart map
  static Map<String, List<UnreadNotifications>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<UnreadNotifications>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = UnreadNotifications.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'unread',
    'requests',
  };
}

