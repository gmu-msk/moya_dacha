//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class GroupRequestList {
  /// Returns a new [GroupRequestList] instance.
  GroupRequestList({
    this.items = const [],
  });

  List<GroupRequest> items;

  @override
  bool operator ==(Object other) => identical(this, other) || other is GroupRequestList &&
    _deepEquality.equals(other.items, items);

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (items.hashCode);

  @override
  String toString() => 'GroupRequestList[items=$items]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'items'] = this.items;
    return json;
  }

  /// Returns a new [GroupRequestList] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static GroupRequestList? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'items'), 'Required key "GroupRequestList[items]" is missing from JSON.');
        assert(json[r'items'] != null, 'Required key "GroupRequestList[items]" has a null value in JSON.');
        return true;
      }());

      return GroupRequestList(
        items: GroupRequest.listFromJson(json[r'items']),
      );
    }
    return null;
  }

  static List<GroupRequestList> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <GroupRequestList>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = GroupRequestList.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, GroupRequestList> mapFromJson(dynamic json) {
    final map = <String, GroupRequestList>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = GroupRequestList.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of GroupRequestList-objects as value to a dart map
  static Map<String, List<GroupRequestList>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<GroupRequestList>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = GroupRequestList.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'items',
  };
}

