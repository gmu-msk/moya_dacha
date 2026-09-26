//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//
// @dart=2.18

// ignore_for_file: unused_element, unused_import
// ignore_for_file: always_put_required_named_parameters_first
// ignore_for_file: constant_identifier_names
// ignore_for_file: lines_longer_than_80_chars

part of openapi.api;

class Feedback {
  /// Returns a new [Feedback] instance.
  Feedback({
    required this.id,
    required this.text,
    required this.status,
    this.issue,
    this.build,
    required this.createdAt,
  });

  int id;

  String text;

  /// `sent` — записан, задачи ещё нет; `accepted` — задача заведена; `approved` — одобрено; `declined` — делать не будем; `done` — сделано, ждёт сборки; `released` — вышло в сборке `build`. 
  FeedbackStatusEnum status;

  /// Номер задачи GitHub, когда она заведена
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  int? issue;

  /// Только у `released` — номер сборки, в которой вышло
  ///
  /// Please note: This property should have been non-nullable! Since the specification file
  /// does not include a default value (using the "default:" property), however, the generated
  /// source code must fall back to having a nullable type.
  /// Consider adding a "default:" property in the specification file to hide this note.
  ///
  int? build;

  DateTime createdAt;

  @override
  bool operator ==(Object other) => identical(this, other) || other is Feedback &&
    other.id == id &&
    other.text == text &&
    other.status == status &&
    other.issue == issue &&
    other.build == build &&
    other.createdAt == createdAt;

  @override
  int get hashCode =>
    // ignore: unnecessary_parenthesis
    (id.hashCode) +
    (text.hashCode) +
    (status.hashCode) +
    (issue == null ? 0 : issue!.hashCode) +
    (build == null ? 0 : build!.hashCode) +
    (createdAt.hashCode);

  @override
  String toString() => 'Feedback[id=$id, text=$text, status=$status, issue=$issue, build=$build, createdAt=$createdAt]';

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};
      json[r'id'] = this.id;
      json[r'text'] = this.text;
      json[r'status'] = this.status;
    if (this.issue != null) {
      json[r'issue'] = this.issue;
    } else {
      json[r'issue'] = null;
    }
    if (this.build != null) {
      json[r'build'] = this.build;
    } else {
      json[r'build'] = null;
    }
      json[r'created_at'] = this.createdAt.toUtc().toIso8601String();
    return json;
  }

  /// Returns a new [Feedback] instance and imports its values from
  /// [value] if it's a [Map], null otherwise.
  // ignore: prefer_constructors_over_static_methods
  static Feedback? fromJson(dynamic value) {
    if (value is Map) {
      final json = value.cast<String, dynamic>();

      // Ensure that the map contains the required keys.
      // Note 1: the values aren't checked for validity beyond being non-null.
      // Note 2: this code is stripped in release mode!
      assert(() {
        assert(json.containsKey(r'id'), 'Required key "Feedback[id]" is missing from JSON.');
        assert(json[r'id'] != null, 'Required key "Feedback[id]" has a null value in JSON.');
        assert(json.containsKey(r'text'), 'Required key "Feedback[text]" is missing from JSON.');
        assert(json[r'text'] != null, 'Required key "Feedback[text]" has a null value in JSON.');
        assert(json.containsKey(r'status'), 'Required key "Feedback[status]" is missing from JSON.');
        assert(json[r'status'] != null, 'Required key "Feedback[status]" has a null value in JSON.');
        assert(json.containsKey(r'created_at'), 'Required key "Feedback[created_at]" is missing from JSON.');
        assert(json[r'created_at'] != null, 'Required key "Feedback[created_at]" has a null value in JSON.');
        return true;
      }());

      return Feedback(
        id: mapValueOfType<int>(json, r'id')!,
        text: mapValueOfType<String>(json, r'text')!,
        status: FeedbackStatusEnum.fromJson(json[r'status'])!,
        issue: mapValueOfType<int>(json, r'issue'),
        build: mapValueOfType<int>(json, r'build'),
        createdAt: mapDateTime(json, r'created_at', r'')!,
      );
    }
    return null;
  }

  static List<Feedback> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <Feedback>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = Feedback.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }

  static Map<String, Feedback> mapFromJson(dynamic json) {
    final map = <String, Feedback>{};
    if (json is Map && json.isNotEmpty) {
      json = json.cast<String, dynamic>(); // ignore: parameter_assignments
      for (final entry in json.entries) {
        final value = Feedback.fromJson(entry.value);
        if (value != null) {
          map[entry.key] = value;
        }
      }
    }
    return map;
  }

  // maps a json object with a list of Feedback-objects as value to a dart map
  static Map<String, List<Feedback>> mapListFromJson(dynamic json, {bool growable = false,}) {
    final map = <String, List<Feedback>>{};
    if (json is Map && json.isNotEmpty) {
      // ignore: parameter_assignments
      json = json.cast<String, dynamic>();
      for (final entry in json.entries) {
        map[entry.key] = Feedback.listFromJson(entry.value, growable: growable,);
      }
    }
    return map;
  }

  /// The list of required keys that must be present in a JSON.
  static const requiredKeys = <String>{
    'id',
    'text',
    'status',
    'created_at',
  };
}

/// `sent` — записан, задачи ещё нет; `accepted` — задача заведена; `approved` — одобрено; `declined` — делать не будем; `done` — сделано, ждёт сборки; `released` — вышло в сборке `build`. 
enum FeedbackStatusEnum {
  sent._(r'sent'),
  accepted._(r'accepted'),
  approved._(r'approved'),
  declined._(r'declined'),
  done._(r'done'),
  released._(r'released'),
  ;

  /// Instantiate a new enum with the provided value.
  const FeedbackStatusEnum._(this._value);

  /// The underlying value of this enum member.
  final String _value;

  @override
  String toString() => _value;

  /// Encodes this enum as a value suitable for JSON.
  String toJson() => _value;

  /// Returns the instance of [FeedbackStatusEnum] that was successfully decoded
  /// from the passed [value] on success, null otherwise.
  static FeedbackStatusEnum? fromJson(dynamic value) => FeedbackStatusEnumTypeTransformer().decode(value);

  /// Returns a [List] containing instances of [FeedbackStatusEnum]
  /// that were successfully decoded from the passed [JSON][json].
  static List<FeedbackStatusEnum> listFromJson(dynamic json, {bool growable = false,}) {
    final result = <FeedbackStatusEnum>[];
    if (json is List && json.isNotEmpty) {
      for (final row in json) {
        final value = FeedbackStatusEnum.fromJson(row);
        if (value != null) {
          result.add(value);
        }
      }
    }
    return result.toList(growable: growable);
  }
}

/// Transformation class that can [encode] an instance of [FeedbackStatusEnum] to String,
/// and [decode] dynamic data back to [FeedbackStatusEnum].
class FeedbackStatusEnumTypeTransformer {
  factory FeedbackStatusEnumTypeTransformer() => _instance ??= const FeedbackStatusEnumTypeTransformer._();

  const FeedbackStatusEnumTypeTransformer._();

  String encode(FeedbackStatusEnum data) => data._value;

  /// Returns the instance of [FeedbackStatusEnum] that was successfully decoded
  /// from the passed [data] value on success, null otherwise.
  ///
  /// If [allowNull] is true and the [dynamic value][data] cannot be decoded successfully,
  /// then null is returned. However, if [allowNull] is false and the [dynamic value][data]
  /// cannot be decoded successfully, then an [UnimplementedError] is thrown.
  ///
  /// The [allowNull] is very handy when an API changes and a new enum value is added or removed,
  /// and users are still using an old app with the old code.
  FeedbackStatusEnum? decode(dynamic data, {bool allowNull = true}) {
    if (data is FeedbackStatusEnum) {
      return data;
    }
    if (data != null) {
      switch (data) {
        case r'sent': return FeedbackStatusEnum.sent;
        case r'accepted': return FeedbackStatusEnum.accepted;
        case r'approved': return FeedbackStatusEnum.approved;
        case r'declined': return FeedbackStatusEnum.declined;
        case r'done': return FeedbackStatusEnum.done;
        case r'released': return FeedbackStatusEnum.released;
        default:
          if (!allowNull) {
            throw ArgumentError('Unknown enum value to decode: $data');
          }
      }
    }
    return null;
  }

  /// The singleton instance of this transformer.
  static FeedbackStatusEnumTypeTransformer? _instance;
}


