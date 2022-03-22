Name:           docker-volume-s3
Version:        %{VERSION}
Release:        0%{?dist}
Summary:        Docker volume plugin to support S3 storage backends
Group:          System Environment/Daemons
License:        MIT License
URL:            https://www.aventer.biz

Obsoletes:  docker-volume-s3 < %{version}-%{release}
Provides:	 docker-volume-s3 = %{version}-%{release}
BuildRoot:      %{_tmppath}/%{name}-%{version}-%{release}-root-%(%{__id_u} -n)

%description
Docker volume plugin to support S3 storage backends


%install
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/usr/bin/
mkdir -p $RPM_BUILD_ROOT/etc/docker-volume/
mkdir -p $RPM_BUILD_ROOT/usr/lib/systemd/system

cp /root/docker-volume-s3/build/docker-volume-s3 $RPM_BUILD_ROOT/usr/bin/docker-volume-s3
cp /root/docker-volume-s3/build/docker-volume-s3.service $RPM_BUILD_ROOT/usr/lib/systemd/system/docker-volume-s3.service
cp /root/docker-volume-s3/build/s3.env $RPM_BUILD_ROOT/etc/docker-volume/s3.env
chmod +x $RPM_BUILD_ROOT/usr/bin/docker-volume-s3

%files
/usr/bin/docker-volume-s3
/etc/docker-volume/s3.env
/usr/lib/systemd/system/docker-volume-s3.service
