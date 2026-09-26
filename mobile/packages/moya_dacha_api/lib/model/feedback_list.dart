//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class FeedbackList {
  /// Returns a new [FeedbackList] instance.
  FeedbackList({
    this.items = const [],
  });

  /// Отзывы, новые сверху
  List<Feedback> items;

  @override
  bool operator ==(Object other) => identical(this, other) || other is FeedbackList &&
    _deepEquality.equals(other.items, items);

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (items.hashCode);

  @override
  String toString() => 'FeedbackList[items=$items]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'items'] = this.items;
    return json;
  }

  /// Returns a new [FeedbackList] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static FeedbackList? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'items'), 'Required key "FeedbackList[items]" is missing from JSON.');
        assert(json[r'items'] != null, 'Required key "FeedbackList[items]" has a null value in JSON.');
        return true;
      }());

      return FeedbackList(
        items: Feedback.listFromJson(json[r'items']),
      );
    }
    return null;
  }

  static List<FeedbackList> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <FeedbackList>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = FeedbackList.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, FeedbackList> mapFromJson(dynamic json) {
    final map = <String, FeedbackList>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = FeedbackList.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of FeedbackList-objects as value to a dart map
  static Map<String, List<FeedbackList>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<FeedbackList>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = FeedbackList.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'items',
  };
}

