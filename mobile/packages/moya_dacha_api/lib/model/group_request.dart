//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class GroupRequest {
  /// Returns a new [GroupRequest] instance.
  GroupRequest({
    required this.kind,
    required this.group,
    required this.user,
    required this.createdAt,
  });

  GroupRequestKindEnum kind;

  GroupBrief group;

  Author user;

  DateTime createdAt;

  @override
  bool operator ==(Object other) => identical(this, other) || other is GroupRequest &&
    other.kind == kind &&
    other.group == group &&
    other.user == user &&
    other.createdAt == createdAt;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (kind.hashCode) +
    (group.hashCode) +
    (user.hashCode) +
    (createdAt.hashCode);

  @override
  String toString() => 'GroupRequest[kind=$kind, group=$group, user=$user, createdAt=$createdAt]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'kind'] = this.kind;
      json[r'group'] = this.group;
      json[r'user'] = this.user;
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
    return json;
  }

  /// Returns a new [GroupRequest] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static GroupRequest? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'kind'), 'Required key "GroupRequest[kind]" is missing from JSON.');
        assert(json[r'kind'] != null, 'Required key "GroupRequest[kind]" has a null value in JSON.');
        assert(json.containsKey(r'group'), 'Required key "GroupRequest[group]" is missing from JSON.');
        assert(json[r'group'] != null, 'Required key "GroupRequest[group]" has a null value in JSON.');
        assert(json.containsKey(r'user'), 'Required key "GroupRequest[user]" is missing from JSON.');
        assert(json[r'user'] != null, 'Required key "GroupRequest[user]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "GroupRequest[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "GroupRequest[created_at]" has a null value in JSON.');
        return true;
      }());

      return GroupRequest(
        kind: GroupRequestKindEnum.fromJson(json[r'kind'])!,
        group: GroupBrief.fromJson(json[r'group'])!,
        user: Author.fromJson(json[r'user'])!,
        createdAt: mapDateTime(json, r'created_at', r'')!,
      );
    }
    return null;
  }

  static List<GroupRequest> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupRequest>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupRequest.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, GroupRequest> mapFromJson(dynamic json) {
    final map = <String, GroupRequest>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = GroupRequest.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of GroupRequest-objects as value to a dart map
  static Map<String, List<GroupRequest>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<GroupRequest>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = GroupRequest.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'kind',
    'group',
    'user',
    'created_at',
  };
}


enum GroupRequestKindEnum {
  request._(r'request'),
  invite._(r'invite'),
  ;

  /// Instantiate a new enum with the provided value.
  const GroupRequestKindEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [GroupRequestKindEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static GroupRequestKindEnum? fromJson(dynamic value) => GroupRequestKindEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [GroupRequestKindEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<GroupRequestKindEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupRequestKindEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupRequestKindEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [GroupRequestKindEnum] to String,
/// and [decode] dynamic data back to [GroupRequestKindEnum].
class GroupRequestKindEnumTypeTransformer {
  factory GroupRequestKindEnumTypeTransformer() => _instance ??= const GroupRequestKindEnumTypeTransformer._();

  const GroupRequestKindEnumTypeTransformer._();

  String encode(GroupRequestKindEnum data) => data._value;

  /// Returns the instance of [GroupRequestKindEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  GroupRequestKindEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is GroupRequestKindEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'request': return GroupRequestKindEnum.request;
        case r'invite': return GroupRequestKindEnum.invite;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static GroupRequestKindEnumTypeTransformer? _instance;
}


