//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class SessionRequest {
  /// Returns a new [SessionRequest] instance.
  SessionRequest({
    required this.phone,
    required this.code,
  });

  /// Тот же номер, на который запрашивали код
  String phone;

  /// Код подтверждения, четыре цифры
  String code;

  @override
  bool operator ==(Object other) => identical(this, other) || other is SessionRequest &&
    other.phone == phone &&
    other.code == code;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (phone.hashCode) +
    (code.hashCode);

  @override
  String toString() => 'SessionRequest[phone=$phone, code=$code]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'phone'] = this.phone;
      json[r'code'] = this.code;
    return json;
  }

  /// Returns a new [SessionRequest] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static SessionRequest? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'phone'), 'Required key "SessionRequest[phone]" is missing from JSON.');
        assert(json[r'phone'] != null, 'Required key "SessionRequest[phone]" has a null value in JSON.');
        assert(json.containsKey(r'code'), 'Required key "SessionRequest[code]" is missing from JSON.');
        assert(json[r'code'] != null, 'Required key "SessionRequest[code]" has a null value in JSON.');
        return true;
      }());

      return SessionRequest(
        phone: mapValueOfType<String>(json, r'phone')!,
        code: mapValueOfType<String>(json, r'code')!,
      );
    }
    return null;
  }

  static List<SessionRequest> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <SessionRequest>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = SessionRequest.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, SessionRequest> mapFromJson(dynamic json) {
    final map = <String, SessionRequest>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = SessionRequest.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of SessionRequest-objects as value to a dart map
  static Map<String, List<SessionRequest>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<SessionRequest>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = SessionRequest.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'phone',
    'code',
  };
}

