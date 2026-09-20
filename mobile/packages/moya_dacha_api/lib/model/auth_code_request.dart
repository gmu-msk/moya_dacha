//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class AuthCodeRequest {
  /// Returns a new [AuthCodeRequest] instance.
  AuthCodeRequest({
    required this.phone,
  });

  /// Номер телефона в любом привычном виде: `+7 900 123-45-67`, `8 900 123 45 67`, `79001234567` — это один и тот же номер. 
  String phone;

  @override
  bool operator ==(Object other) => identical(this, other) || other is AuthCodeRequest &&
    other.phone == phone;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (phone.hashCode);

  @override
  String toString() => 'AuthCodeRequest[phone=$phone]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'phone'] = this.phone;
    return json;
  }

  /// Returns a new [AuthCodeRequest] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static AuthCodeRequest? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'phone'), 'Required key "AuthCodeRequest[phone]" is missing from JSON.');
        assert(json[r'phone'] != null, 'Required key "AuthCodeRequest[phone]" has a null value in JSON.');
        return true;
      }());

      return AuthCodeRequest(
        phone: mapValueOfType<String>(json, r'phone')!,
      );
    }
    return null;
  }

  static List<AuthCodeRequest> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <AuthCodeRequest>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = AuthCodeRequest.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, AuthCodeRequest> mapFromJson(dynamic json) {
    final map = <String, AuthCodeRequest>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = AuthCodeRequest.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of AuthCodeRequest-objects as value to a dart map
  static Map<String, List<AuthCodeRequest>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<AuthCodeRequest>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = AuthCodeRequest.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'phone',
  };
}

