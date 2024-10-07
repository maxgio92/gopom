package gopom

import (
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"io"
	"os"

	"github.com/pkg/errors"
)

func Parse(path string) (*Project, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, _ := io.ReadAll(file)
	var project Project

	err = xml.Unmarshal(b, &project)
	if err != nil {
		return nil, err
	}

	// Create a dependency cache for O(1) search.
	if project.Dependencies != nil {
		project.depCache = make(map[uint64]*Dependency, len(*project.Dependencies))
		for _, dep := range *project.Dependencies {
			dep := dep
			hash, err := depHashF(dep.GroupID, dep.ArtifactID)
			if err != nil {
				return nil, errors.Wrap(err, "failed to calculate hash for dependency")
			}
			project.depCache[*hash] = &dep
		}
	}

	return &project, nil
}

// Marshal marshals the Project struct into a byte slice.
// Note that you must use this to get the correct XML output, as the
// attributes require special handling. Or there's a bug in the struct
// definition, but I don't know what it is.
func (p *Project) Marshal() ([]byte, error) {
	// Set these appropriately for marshalling purposes.
	p.SchemaLocationXSI = p.SchemaLocation
	p.XsiNS = p.Xsi
	p.SchemaLocation = ""
	p.Xsi = ""
	marshalled, err := xml.MarshalIndent(p, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}
	header := []byte(xml.Header)
	return append(header, marshalled...), nil
}

type Project struct {
	XMLName xml.Name `xml:"project,omitempty"`
	Xmlns   string   `xml:"xmlns,attr"`
	Xsi     string   `xml:"xsi,attr,omitempty"`
	// This is the variant of the above. In the POM, it's `xmlns:xsi`,
	// it gets parsed into `xsi`, so we have this so that when we
	// marshal the attribute has the right name. I'm sure it's something that's
	// wrong, but I don't know how to fix this.
	// Previous version had entirely separate struct for Marshalling,
	XsiNS          string `xml:"xmlns:xsi,attr,omitempty"`
	SchemaLocation string `xml:"schemaLocation,attr,omitempty"`
	// This is the variant of the above. In the POM, it's `xsi:schemaLocation`,
	// it gets parsed into `schemaLocation`, so we have this so that when we
	// marshal the attribute has the right name. I'm sure it's something that's
	// wrong, but I don't know how to fix this.
	SchemaLocationXSI      string                  `xml:"xsi:schemaLocation,attr,omitempty"`
	ModelVersion           string                  `xml:"modelVersion,omitempty"`
	GroupID                string                  `xml:"groupId,omitempty"`
	ArtifactID             string                  `xml:"artifactId,omitempty"`
	Version                string                  `xml:"version,omitempty"`
	Packaging              string                  `xml:"packaging,omitempty"`
	Name                   string                  `xml:"name,omitempty"`
	Description            string                  `xml:"description,omitempty"`
	URL                    string                  `xml:"url,omitempty"`
	InceptionYear          string                  `xml:"inceptionYear,omitempty"`
	Organization           *Organization           `xml:"organization,omitempty"`
	Licenses               *[]License              `xml:"licenses>license,omitempty"`
	Developers             *[]Developer            `xml:"developers>developer,omitempty"`
	Contributors           *[]Contributor          `xml:"contributors>contributor,omitempty"`
	MailingLists           *[]MailingList          `xml:"mailingLists>mailingList,omitempty"`
	Prerequisites          *Prerequisites          `xml:"prerequisites,omitempty"`
	Properties             *Properties             `xml:"properties,omitempty"`
	Parent                 *Parent                 `xml:"parent,omitempty"`
	Modules                *[]string               `xml:"modules>module,omitempty"`
	SCM                    *Scm                    `xml:"scm,omitempty"`
	IssueManagement        *IssueManagement        `xml:"issueManagement,omitempty"`
	CIManagement           *CIManagement           `xml:"ciManagement,omitempty"`
	DistributionManagement *DistributionManagement `xml:"distributionManagement,omitempty"`
	DependencyManagement   *DependencyManagement   `xml:"dependencyManagement,omitempty"`
	Dependencies           *[]Dependency           `xml:"dependencies>dependency,omitempty"`
	Repositories           *[]Repository           `xml:"repositories>repository,omitempty"`
	PluginRepositories     *[]PluginRepository     `xml:"pluginRepositories>pluginRepository,omitempty"`
	Build                  *Build                  `xml:"build,omitempty"`
	Reporting              *Reporting              `xml:"reporting,omitempty"`
	Profiles               *[]Profile              `xml:"profiles>profile,omitempty"`

	depCache map[uint64]*Dependency
}

type Properties struct {
	Entries map[string]string
	// To preserve ordering when marshalling, we need to keep track of the
	// order. I'm sure there's a better way to do this, but this will suffice
	// for now.
	Order []string
}

func (p *Properties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	type entry struct {
		XMLName xml.Name
		Key     string `xml:"name,attr"`
		Value   string `xml:",chardata"`
	}
	e := entry{}
	p.Entries = map[string]string{}
	for err = d.Decode(&e); err == nil; err = d.Decode(&e) {
		e.Key = e.XMLName.Local
		p.Entries[e.Key] = e.Value
		p.Order = append(p.Order, e.Key)
	}
	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

func (p Properties) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	tokens := []xml.Token{start}

	for _, name := range p.Order {
		t := xml.StartElement{Name: xml.Name{"", name}}
		tokens = append(tokens, t, xml.CharData(p.Entries[name]), xml.EndElement{t.Name})
	}

	tokens = append(tokens, xml.EndElement{start.Name})

	for _, t := range tokens {
		err := e.EncodeToken(t)
		if err != nil {
			return err
		}
	}

	return e.Flush()
}

// Search returns the first match of a dependency keyed by artifactID and groupID.
func (p *Project) Search(groupId, artifactId string) (*Dependency, error) {
	hash, err := depHashF(groupId, artifactId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hash dependency key")
	}
	if hash == nil {
		return nil, errors.Wrap(err, "failed to hash dependency key")
	}
	if p.depCache == nil {
		return nil, ErrDepCacheEmpty
	}

	dep, ok := p.depCache[*hash]
	if !ok {
		return nil, ErrDepNotFound
	}

	return dep, nil
}

func depHashF(groupID, artifactID string) (*uint64, error) {
	f := fnv.New64a()
	_, err := f.Write([]byte(fmt.Sprintf("%s:%s", groupID, artifactID)))
	if err != nil {
		return nil, err
	}

	hash := f.Sum64()

	return &hash, nil
}

type Parent struct {
	GroupID      string `xml:"groupId,omitempty"`
	ArtifactID   string `xml:"artifactId,omitempty"`
	Version      string `xml:"version,omitempty"`
	RelativePath string `xml:"relativePath,omitempty"`
}

type Organization struct {
	Name string `xml:"name,omitempty"`
	URL  string `xml:"url,omitempty"`
}

type License struct {
	Name         string `xml:"name,omitempty"`
	URL          string `xml:"url,omitempty"`
	Distribution string `xml:"distribution,omitempty"`
	Comments     string `xml:"comments,omitempty"`
}

type Developer struct {
	ID              string      `xml:"id,omitempty"`
	Name            string      `xml:"name,omitempty"`
	Email           string      `xml:"email,omitempty"`
	URL             string      `xml:"url,omitempty"`
	Organization    string      `xml:"organization,omitempty"`
	OrganizationURL string      `xml:"organizationUrl,omitempty"`
	Roles           *[]string   `xml:"roles>role,omitempty"`
	Timezone        string      `xml:"timezone,omitempty"`
	Properties      *Properties `xml:"properties,omitempty"`
}

type Contributor struct {
	Name            string      `xml:"name,omitempty"`
	Email           string      `xml:"email,omitempty"`
	URL             string      `xml:"url,omitempty"`
	Organization    string      `xml:"organization,omitempty"`
	OrganizationURL string      `xml:"organizationUrl,omitempty"`
	Roles           *[]string   `xml:"roles>role,omitempty"`
	Timezone        string      `xml:"timezone,omitempty"`
	Properties      *Properties `xml:"properties,omitempty"`
}

type MailingList struct {
	Name          string    `xml:"name,omitempty"`
	Subscribe     string    `xml:"subscribe,omitempty"`
	Unsubscribe   string    `xml:"unsubscribe,omitempty"`
	Post          string    `xml:"post,omitempty"`
	Archive       string    `xml:"archive,omitempty"`
	OtherArchives *[]string `xml:"otherArchives>otherArchive,omitempty"`
}

type Prerequisites struct {
	Maven string `xml:"maven,omitempty"`
}

type Scm struct {
	Connection          string `xml:"connection,omitempty"`
	DeveloperConnection string `xml:"developerConnection,omitempty"`
	Tag                 string `xml:"tag,omitempty"`
	URL                 string `xml:"url,omitempty"`
}

type IssueManagement struct {
	System string `xml:"system,omitempty"`
	URL    string `xml:"url,omitempty"`
}

type CIManagement struct {
	System    string      `xml:"system,omitempty"`
	URL       string      `xml:"url,omitempty"`
	Notifiers *[]Notifier `xml:"notifiers>notifier,omitempty"`
}

type Notifier struct {
	Type          string         `xml:"type,omitempty"`
	SendOnError   bool           `xml:"sendOnError,omitempty"`
	SendOnFailure bool           `xml:"sendOnFailure,omitempty"`
	SendOnSuccess bool           `xml:"sendOnSuccess,omitempty"`
	SendOnWarning bool           `xml:"sendOnWarning,omitempty"`
	Address       string         `xml:"address,omitempty"`
	Configuration *Configuration `xml:"configuration,omitempty"`
}

type DistributionManagement struct {
	Repository         *Repository `xml:"repository,omitempty"`
	SnapshotRepository *Repository `xml:"snapshotRepository,omitempty"`
	Site               *Site       `xml:"site,omitempty"`
	DownloadURL        string      `xml:"downloadUrl,omitempty"`
	Relocation         *Relocation `xml:"relocation,omitempty"`
	Status             string      `xml:"status,omitempty"`
}

type Site struct {
	ID   string `xml:"id,omitempty"`
	Name string `xml:"name,omitempty"`
	URL  string `xml:"url,omitempty"`
}

type Relocation struct {
	GroupID    string `xml:"groupId,omitempty"`
	ArtifactID string `xml:"artifactId,omitempty"`
	Version    string `xml:"version,omitempty"`
	Message    string `xml:"message,omitempty"`
}

type DependencyManagement struct {
	Dependencies *[]Dependency `xml:"dependencies>dependency,omitempty"`
}

type Dependency struct {
	GroupID    string       `xml:"groupId,omitempty"`
	ArtifactID string       `xml:"artifactId,omitempty"`
	Version    string       `xml:"version,omitempty"`
	Type       string       `xml:"type,omitempty"`
	Classifier string       `xml:"classifier,omitempty"`
	Scope      string       `xml:"scope,omitempty"`
	SystemPath string       `xml:"systemPath,omitempty"`
	Exclusions *[]Exclusion `xml:"exclusions>exclusion,omitempty"`
	Optional   string       `xml:"optional,omitempty"`
}

type Exclusion struct {
	GroupID    string `xml:"groupId,omitempty"`
	ArtifactID string `xml:"artifactId,omitempty"`
}

type Repository struct {
	UniqueVersion bool              `xml:"uniqueVersion,omitempty"`
	Releases      *RepositoryPolicy `xml:"releases,omitempty"`
	Snapshots     *RepositoryPolicy `xml:"snapshots,omitempty"`
	ID            string            `xml:"id,omitempty"`
	Name          string            `xml:"name,omitempty"`
	URL           string            `xml:"url,omitempty"`
	Layout        string            `xml:"layout,omitempty"`
}

type RepositoryPolicy struct {
	Enabled        string `xml:"enabled,omitempty"`
	UpdatePolicy   string `xml:"updatePolicy,omitempty"`
	ChecksumPolicy string `xml:"checksumPolicy,omitempty"`
}

type PluginRepository struct {
	Releases  *RepositoryPolicy `xml:"releases,omitempty"`
	Snapshots *RepositoryPolicy `xml:"snapshots,omitempty"`
	ID        string            `xml:"id,omitempty"`
	Name      string            `xml:"name,omitempty"`
	URL       string            `xml:"url,omitempty"`
	Layout    string            `xml:"layout,omitempty"`
}

type BuildBase struct {
	DefaultGoal      string            `xml:"defaultGoal,omitempty"`
	Resources        *[]Resource       `xml:"resources>resource,omitempty"`
	TestResources    *[]Resource       `xml:"testResources>testResource,omitempty"`
	Directory        string            `xml:"directory,omitempty"`
	FinalName        string            `xml:"finalName,omitempty"`
	Filters          *[]string         `xml:"filters>filter,omitempty"`
	PluginManagement *PluginManagement `xml:"pluginManagement,omitempty"`
	Plugins          *[]Plugin         `xml:"plugins>plugin,omitempty"`
}

type Build struct {
	SourceDirectory       string       `xml:"sourceDirectory,omitempty"`
	ScriptSourceDirectory string       `xml:"scriptSourceDirectory,omitempty"`
	TestSourceDirectory   string       `xml:"testSourceDirectory,omitempty"`
	OutputDirectory       string       `xml:"outputDirectory,omitempty"`
	TestOutputDirectory   string       `xml:"testOutputDirectory,omitempty"`
	Extensions            *[]Extension `xml:"extensions>extension,omitempty"`
	BuildBase
}

type Extension struct {
	GroupID    string `xml:"groupId,omitempty"`
	ArtifactID string `xml:"artifactId,omitempty"`
	Version    string `xml:"version,omitempty"`
}

type Resource struct {
	TargetPath string    `xml:"targetPath,omitempty"`
	Filtering  string    `xml:"filtering,omitempty"`
	Directory  string    `xml:"directory,omitempty"`
	Includes   *[]string `xml:"includes>include,omitempty"`
	Excludes   *[]string `xml:"excludes>exclude,omitempty"`
}

type PluginManagement struct {
	Plugins *[]Plugin `xml:"plugins>plugin,omitempty"`
}

// Configuration is a raw XML configuration that we currently do not muck with.
// It's supposed to be a DOM object, but upstream uses map[string]string for
// properties, and it does not work. For now, just keep it as a string so we
// can marshal it out untouched.
// TODO: This should be a DOM object.
type Configuration struct {
	Children         string `xml:"combine.children,attr,omitempty"`
	Self             string `xml:"combine.self,attr,omitempty"`
	RawConfiguration string `xml:",innerxml"`
}

type Plugin struct {
	GroupID       string             `xml:"groupId,omitempty"`
	ArtifactID    string             `xml:"artifactId,omitempty"`
	Version       string             `xml:"version,omitempty"`
	Extensions    string             `xml:"extensions,omitempty"`
	Executions    *[]PluginExecution `xml:"executions>execution,omitempty"`
	Dependencies  *[]Dependency      `xml:"dependencies>dependency,omitempty"`
	Inherited     string             `xml:"inherited,omitempty"`
	Configuration *Configuration     `xml:"configuration,omitempty"`
}

type PluginExecution struct {
	ID            string         `xml:"id,omitempty"`
	Phase         string         `xml:"phase,omitempty"`
	Goals         *[]string      `xml:"goals>goal,omitempty"`
	Inherited     string         `xml:"inherited,omitempty"`
	Configuration *Configuration `xml:"configuration,omitempty"`
}

type Reporting struct {
	ExcludeDefaults string             `xml:"excludeDefaults,omitempty"`
	OutputDirectory string             `xml:"outputDirectory,omitempty"`
	Plugins         *[]ReportingPlugin `xml:"plugins>plugin,omitempty"`
}

type ReportingPlugin struct {
	GroupID    string       `xml:"groupId,omitempty"`
	ArtifactID string       `xml:"artifactId,omitempty"`
	Version    string       `xml:"version,omitempty"`
	Inherited  string       `xml:"inherited,omitempty"`
	ReportSets *[]ReportSet `xml:"reportSets>reportSet,omitempty"`
}

type ReportSet struct {
	ID        string    `xml:"id,omitempty"`
	Reports   *[]string `xml:"reports>report,omitempty"`
	Inherited string    `xml:"inherited,omitempty"`
}

type Profile struct {
	ID                     string                  `xml:"id,omitempty"`
	Activation             *Activation             `xml:"activation,omitempty"`
	Build                  *BuildBase              `xml:"build,omitempty"`
	Modules                *[]string               `xml:"modules>module,omitempty"`
	DistributionManagement *DistributionManagement `xml:"distributionManagement,omitempty"`
	Properties             *Properties             `xml:"properties,omitempty"`
	DependencyManagement   *DependencyManagement   `xml:"dependencyManagement,omitempty"`
	Dependencies           *[]Dependency           `xml:"dependencies>dependency,omitempty"`
	Repositories           *[]Repository           `xml:"repositories>repository,omitempty"`
	PluginRepositories     *[]PluginRepository     `xml:"pluginRepositories>pluginRepository,omitempty"`
	Reporting              *Reporting              `xml:"reporting,omitempty"`
}

type Activation struct {
	ActiveByDefault bool                `xml:"activeByDefault,omitempty"`
	JDK             string              `xml:"jdk,omitempty"`
	OS              *ActivationOS       `xml:"os,omitempty"`
	Property        *ActivationProperty `xml:"property,omitempty"`
	File            *ActivationFile     `xml:"file,omitempty"`
}

type ActivationOS struct {
	Name    string `xml:"name,omitempty"`
	Family  string `xml:"family,omitempty"`
	Arch    string `xml:"arch,omitempty"`
	Version string `xml:"version,omitempty"`
}

type ActivationProperty struct {
	Name  string `xml:"name,omitempty"`
	Value string `xml:"value,omitempty"`
}

type ActivationFile struct {
	Missing string `xml:"missing,omitempty"`
	Exists  string `xml:"exists,omitempty"`
}
